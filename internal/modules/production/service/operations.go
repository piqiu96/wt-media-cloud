package service

import (
	transferdto "github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/dto"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
)

func GetMaterial(actor identityservice.PublicUser, materialID int64) (model.Material, error) {
	return newWiredService().GetMaterial(actor, materialID)
}

func AddUsage(actor identityservice.PublicUser, materialID int64) (model.MaterialUsage, bool, error) {
	return newWiredService().AddUsage(actor, materialID)
}

// newWiredService assembles the module's real dependencies, in one place. It
// exists because there are three of them now, and repeating the triple at each
// wrapper is how one of them eventually goes missing from one call site.
func newWiredService() *Service {
	return NewService(mysqlStore{}, runtimeNodeResolver{}, productionTransferCreator{})
}

func CreateDownload(actor identityservice.PublicUser, materialID int64) (transferdto.Task, error) {
	return newWiredService().CreateDownload(actor, materialID)
}

func ListMaterials(actor identityservice.PublicUser, search string) ([]model.Material, error) {
	return newWiredService().ListMaterials(actor, search)
}

func ListMyMaterials(actor identityservice.PublicUser) ([]model.MaterialUsage, error) {
	return newWiredService().ListMyMaterials(actor)
}

func RemoveUsage(actor identityservice.PublicUser, usageID int64) error {
	return newWiredService().RemoveUsage(actor, usageID)
}

// MarkVideoReady and MarkVideoFailed are the Cloud worker's entry points, and the
// only functions in this module that no actor reaches. They are declared here
// beside the actor-facing ones so that the module's public surface is one list
// rather than two files a reader has to know to look in.
func MarkVideoReady(teamID identityservice.TeamID, materialID int64, facts repository.VideoFacts) error {
	return newWiredService().MarkVideoReady(teamID, materialID, facts)
}

func MarkVideoFailed(teamID identityservice.TeamID, materialID int64, message string) error {
	return newWiredService().MarkVideoFailed(teamID, materialID, message)
}
