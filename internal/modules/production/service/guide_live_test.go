package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/infra/storage"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/compose"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/executor"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
)

func TestLiveGuideDownloadComposeAndStorageReadback(t *testing.T) {
	if os.Getenv("WT_MEDIA_LIVE_GUIDE") != "1" {
		t.Skip("set WT_MEDIA_LIVE_GUIDE=1 for configured object storage")
	}
	root, err := filepath.Abs("../../../../")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(config.HomeEnvVar, root)
	config.MustInitializeRuntimePaths()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.Initialize(cfg.Storage.ObjectStorage, cfg.Credentials.ObjectStorage); err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	dir := t.TempDir()
	clip := func(name, color string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		command := exec.CommandContext(ctx, cfg.Media.FFmpeg.FFmpegPath, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "color=c="+color+":s=320x240:r=30", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000", "-t", "2", "-c:v", "libx264", "-c:a", "aac", "-y", path)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("make clip: %v %s", err, output)
		}
		return path
	}
	source, guide := clip("source.mp4", "red"), clip("guide.mp4", "blue")
	store := storage.Get()
	keyPrefix := fmt.Sprintf("compose-verification/chg093-%d", time.Now().UnixNano())
	guideKey, outputKey := keyPrefix+"/guide.mp4", keyPrefix+"/output.mp4"
	defer func() {
		_ = store.Remove(context.Background(), guideKey)
		_ = store.Remove(context.Background(), outputKey)
	}()
	file, err := os.Open(guide)
	if err != nil {
		t.Fatal(err)
	}
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(ctx, guideKey, file, info.Size(), "video/mp4"); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	grant, err := store.PresignGet(ctx, guideKey, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	downloaded := filepath.Join(dir, "downloaded-guide")
	if err := compose.DownloadGuide(ctx, grant.URL, downloaded); err != nil {
		t.Fatal(err)
	}
	registry, err := executor.NewRegistry(executor.DeployedDefinitions()...)
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := compose.NewWorkspace(filepath.Join(dir, "runs"))
	if err != nil {
		t.Fatal(err)
	}
	runDir, err := workspace.NewRun("live-guide")
	if err != nil {
		t.Fatal(err)
	}
	defer runDir.Release()
	params, _ := json.Marshal(map[string]any{"guide_video_url": grant.URL, "insert_seconds": 1, "guide_duration_seconds": 0, "max_duration_seconds": 0})
	runner := compose.Runner{FFmpegPath: cfg.Media.FFmpeg.FFmpegPath, FFprobePath: cfg.Media.FFmpeg.FFprobePath, Registry: registry}
	output, err := runner.Run(ctx, runDir, compose.Request{TemplateCode: "guide_insert", TemplateVersion: 1, Params: params, InputPath: source, GuidePath: downloaded})
	if err != nil {
		t.Fatal(err)
	}
	if output.DurationMS < 3700 || output.DurationMS > 4300 {
		t.Fatalf("duration=%d", output.DurationMS)
	}
	size, digest, err := uploadVerified(ctx, store, output.Path, outputKey)
	if err != nil || size == 0 || len(digest) != 64 {
		t.Fatalf("upload size=%d digest length=%d err=%v", size, len(digest), err)
	}
	if os.Getenv("WT_MEDIA_LIVE_GUIDE_TASK") == "1" {
		verifyLiveGuideTask(t, ctx, cfg, store, grant.URL)
	}
}

// This optional local smoke check requires a running compose Worker and an
// existing ready material in the named operator's My Materials. It leaves a
// completed task/output for UI review and disables the temporary strategy.
func verifyLiveGuideTask(t *testing.T, ctx context.Context, cfg config.Config, store storage.Store, guideURL string) {
	t.Helper()
	materialID, err := strconv.ParseInt(os.Getenv("WT_MEDIA_LIVE_MATERIAL_ID"), 10, 64)
	if err != nil || materialID <= 0 {
		t.Fatal("WT_MEDIA_LIVE_MATERIAL_ID must name a ready My Materials item")
	}
	actorID, err := strconv.ParseInt(os.Getenv("WT_MEDIA_LIVE_ACTOR_ID"), 10, 64)
	if err != nil || actorID <= 0 {
		t.Fatal("WT_MEDIA_LIVE_ACTOR_ID must name the material owner")
	}
	if err := database.Initialize(cfg.Databases); err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	actor, found, err := identity.ResolveUser(identity.UserID(actorID))
	if err != nil || !found {
		t.Fatalf("resolve task actor: found=%v err=%v", found, err)
	}
	admin, found, err := identity.ResolveUser(identity.UserID(1))
	if err != nil || !found {
		t.Fatalf("resolve admin: found=%v err=%v", found, err)
	}
	var candidates []struct {
		ID        int64
		ObjectKey string
	}
	if err := database.DB().Raw("SELECT m.id, m.source_object_key AS object_key FROM materials m JOIN material_usages u ON u.material_id = m.id WHERE u.user_id = ? AND u.status = 'active' AND m.video_status = 'ready' ORDER BY (m.id = ?) DESC, m.id DESC LIMIT 100", actorID, materialID).Scan(&candidates).Error; err != nil {
		t.Fatal(err)
	}
	materialID = 0
	for _, candidate := range candidates {
		if candidate.ObjectKey != "" {
			if _, err := store.Stat(ctx, candidate.ObjectKey); err == nil {
				materialID = candidate.ID
				break
			}
		}
	}
	if materialID == 0 {
		t.Fatal("no ready My Materials item has an existing source object")
	}
	var templateID int64
	if err := database.DB().Raw("SELECT id FROM compose_templates WHERE code = ? AND version = 1 AND status = 'active'", "guide_insert").Scan(&templateID).Error; err != nil || templateID == 0 {
		t.Fatalf("resolve guide template: id=%d err=%v", templateID, err)
	}
	var materialScope struct {
		TeamID     int64
		CategoryID identity.CategoryID
	}
	if err := database.DB().Raw("SELECT m.team_id, s.category_id FROM materials m JOIN source_contents s ON s.id = m.source_content_id WHERE m.id = ?", materialID).Scan(&materialScope).Error; err != nil || materialScope.TeamID == 0 || materialScope.CategoryID <= 0 {
		t.Fatalf("resolve material scope: team=%d category=%d err=%v", materialScope.TeamID, materialScope.CategoryID, err)
	}
	params, err := json.Marshal(map[string]any{"guide_video_url": guideURL, "insert_seconds": 1, "guide_duration_seconds": 1, "max_duration_seconds": 4})
	if err != nil {
		t.Fatal(err)
	}
	strategy, err := CreateComposeStrategy(admin, CreateStrategyInput{TeamID: materialScope.TeamID, CategoryID: materialScope.CategoryID, Name: "CHG093 引导视频链路验证", TemplateID: templateID, TemplateVersion: 1, Config: params, ExecutionMode: model.ComposeExecutionManual, PriorityRules: strategyRulesFixture(), SingleLimit: 1, DailyLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := SetComposeStrategyStatus(admin, strategy.ID, model.ComposeStrategyDisabled); err != nil {
			t.Errorf("disable verification strategy %d: %v", strategy.ID, err)
		}
	}()
	requestKey := fmt.Sprintf("chg093-guide-%d", time.Now().UnixNano())
	created, err := CreateComposeTasks(actor, CreateComposeTasksInput{StrategyID: strategy.ID, Items: []CreateComposeTaskItemInput{{MaterialID: materialID, RequestKey: requestKey}}})
	if err != nil || len(created.Created) != 1 {
		t.Fatalf("create guide task: created=%d rejected=%v err=%v", len(created.Created), created.Failed, err)
	}
	taskID := created.Created[0].ID
	t.Logf("verification strategy=%d task=%d", strategy.ID, taskID)
	for {
		task, err := GetComposeTask(actor, taskID)
		if err != nil {
			t.Fatal(err)
		}
		if task.Status == model.ComposeTaskFailed || task.Status == model.ComposeTaskCancelled {
			t.Fatalf("guide task ended %s: %s %s", task.Status, task.ErrorCode, task.ErrorMessage)
		}
		if task.Status == model.ComposeTaskSuccess {
			if task.CompositeOutput == nil {
				t.Fatal("successful task has no output ID")
			}
			output, err := GetComposeOutput(actor, *task.CompositeOutput)
			if err != nil || output.Output.TaskID != taskID {
				t.Fatalf("read output: task=%d err=%v", output.Output.TaskID, err)
			}
			listed, err := ListComposeOutputs(actor, ListComposeOutputsInput{})
			if err != nil {
				t.Fatal(err)
			}
			visible := false
			for _, item := range listed {
				visible = visible || item.ID == output.Output.ID
			}
			if !visible {
				t.Fatal("output missing from My Outputs")
			}
			preview, err := ComposeOutputVideoURL(ctx, actor, output.Output.ID)
			if err != nil {
				t.Fatal(err)
			}
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, preview, nil)
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Range", "bytes=0-1023")
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			_ = response.Body.Close()
			if response.StatusCode != http.StatusPartialContent && response.StatusCode != http.StatusOK {
				t.Fatalf("preview readback status=%d", response.StatusCode)
			}
			t.Logf("output=%d bytes=%d preview_status=%d", output.Output.ID, output.Output.FileSizeBytes, response.StatusCode)
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("guide task %d did not finish: %v", taskID, ctx.Err())
		case <-time.After(2 * time.Second):
		}
	}
}
