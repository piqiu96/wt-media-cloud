"""CHG109 provider schemas; needs pyyaml, jsonschema, openapi-spec-validator."""
from __future__ import annotations

import copy
import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path

import yaml
from jsonschema import Draft202012Validator, FormatChecker
from openapi_spec_validator import OpenAPIV31SpecValidator
from jsonschema_path import SchemaPath
from referencing import Registry, Resource
from referencing.jsonschema import DRAFT202012

ROOT = Path(__file__).resolve().parents[2]
API = ROOT / "contracts/cloud-api/v1"


class PublicationPlanContractTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.docs = {p.name: yaml.safe_load(p.read_text()) for p in API.glob("*.yaml")}
        cls.plan = cls.docs["publication-plan.openapi.yaml"]
        cls.registry = Registry().with_resources(
            ((API / name).as_uri(), Resource.from_contents(doc, default_specification=DRAFT202012))
            for name, doc in cls.docs.items()
        )

    def schema(self, name, filename="publication-plan.openapi.yaml"):
        return Draft202012Validator(
            {"$ref": f"{(API / filename).as_uri()}#/components/schemas/{name}"},
            registry=self.registry, format_checker=FormatChecker(),
        )

    def test_changed_openapi_documents_validate(self):
        for name in ["publication-plan.openapi.yaml", "publication-batch.openapi.yaml", "publication.openapi.yaml"]:
            with self.subTest(name=name):
                OpenAPIV31SpecValidator(SchemaPath.from_dict(self.docs[name], base_uri=(API / name).as_uri())).validate()
                self.assertEqual(self.docs[name]["info"]["version"], "2026.11.26.2")

    def test_bilibili_content_is_independent_and_baidu_is_compatible(self):
        schema = self.schema("Content", "publication.openapi.yaml")
        baidu = {"description": "既有标题", "cover_file_id": "5", "portrait_cover_file_id": "6", "topics": "", "declaration": "无需声明", "confirmed": True}
        self.assertTrue(schema.is_valid(baidu))
        bili = {"title": "游戏画中画演示", "description": "独立简介", "cover_file_id": "5", "space_cover_file_id": "7", "confirmed": True,
                "bilibili": {"partition_id": 17, "partition_name": "游戏", "tags": ["游戏"], "statement": "内容含营销信息", "visibility": "public", "rights_reserved": False}}
        self.assertTrue(schema.is_valid(bili))
        self.assertFalse(schema.is_valid({**bili, "portrait_cover_file_id": "6"}))
        self.assertFalse(schema.is_valid({**baidu, "bilibili": bili["bilibili"]}))
        self.assertFalse(schema.is_valid({**bili, "bilibili": {**bili["bilibili"], "creation_type": "original"}}))
        self.assertFalse(schema.is_valid({**bili, "bilibili": {**bili["bilibili"], "statement": "内容为转载"}}))

    def test_agent_publication_contracts_validate_and_legacy_defaults_remain_baidu(self):
        for filename in ["publications.openapi.yaml", "compatibility.openapi.yaml"]:
            path = ROOT / "contracts/cloud-agent-api/v1" / filename
            doc = yaml.safe_load(path.read_text())
            with self.subTest(filename=filename):
                OpenAPIV31SpecValidator(SchemaPath.from_dict(doc, base_uri=path.as_uri())).validate()
            if filename == "publications.openapi.yaml":
                capabilities = doc["components"]["schemas"]["SupportedPlatforms"]
                self.assertEqual(capabilities["default"], ["baidu"])
                validator = Draft202012Validator(capabilities)
                self.assertTrue(validator.is_valid(["baidu", "bilibili"]))
                for invalid in [[], ["bilibili", "bilibili"], ["xiaohongshu"]]:
                    self.assertFalse(validator.is_valid(invalid))

    def test_running_list_filter_includes_manual_paused_without_changing_stored_states(self):
        parameters = self.plan["paths"]["/api/v1/publication-plan"]["get"]["parameters"]
        status = next(item for item in parameters if item["name"] == "status")
        self.assertIn("running includes running and paused", status["description"])
        self.assertIn("paused remains an exact", status["description"])
        self.assertEqual(status["schema"]["enum"], ["not_started", "running", "paused", "stopping", "ended"])

    def test_complete_surface_has_no_makeup_or_owner_mutation(self):
        operations = [op["operationId"] for path in self.plan["paths"].values()
                      for method, op in path.items() if method in {"get", "post", "put", "delete"}]
        self.assertEqual(len(operations), 16)
        self.assertEqual(len(set(operations)), 16)
        for path in self.plan["paths"]:
            self.assertNotIn("makeup", path)
            self.assertNotIn("authorize", path)
            self.assertNotIn("advance", path)

    def skeleton(self):
        return {"row_key": "row-a", "name": "第一批", "platform": "baidu", "group_order": 1,
                "batch_order": 1, "targets": [{"category_id": "2", "target_count": 10}]}

    def test_create_and_structure_use_stable_owned_batch_identity(self):
        create = {"request_key": "create-1", "name": "计划", "date_start": "2026-11-25",
                  "date_end": "2026-11-26", "note": "", "goals": [{"platform": "baidu", "category_id": "2", "target_count": 10}],
                  "batches": [self.skeleton()]}
        self.schema("CreatePlanInput").validate(create)
        for forbidden in ["plan_id", "owner_id", "batch_id"]:
            bad = copy.deepcopy(create)
            bad["batches"][0][forbidden] = "9"
            self.assertFalse(self.schema("CreatePlanInput").is_valid(bad), forbidden)
        edit = {k: v for k, v in create.items() if k != "request_key"}
        edit["expected_revision"] = "1"
        edit["batches"][0]["batch_id"] = "7"
        self.schema("StructureInput").validate(edit)
        edit["batches"][0]["batch_id"] = 7
        self.assertFalse(self.schema("StructureInput").is_valid(edit))

    def selection(self):
        return {"batch_id": "7", "expected_revision": "3", "included_row_keys": ["a"],
                "excluded_row_keys": [], "allow_shortfall": True,
                "expected_shortfalls": [{"category_id": "2", "shortfall": 9}]}

    def test_first_start_requires_every_explicit_selection_field(self):
        start = {"request_key": "start-1", "expected_revision": "1", "first_batch_id": "7",
                 "batch_confirmations": [self.selection()]}
        self.schema("StartPlanInput").validate(start)
        for field in self.selection():
            for mode in ["missing", "null"]:
                bad = copy.deepcopy(start)
                if mode == "missing":
                    del bad["batch_confirmations"][0][field]
                else:
                    bad["batch_confirmations"][0][field] = None
                self.assertFalse(self.schema("StartPlanInput").is_valid(bad), (field, mode))
        start["batch_confirmations"][0]["included_row_keys"] = []
        self.assertFalse(self.schema("StartPlanInput").is_valid(start))

    def test_confirmation_requires_fresh_basis_and_fixed_position(self):
        confirm = {"request_key": "next-1", "expected_revision": "4", "current_batch_id": "7", "next_batch_id": "8",
                   "expected_basis_fingerprint": "a" * 64, "decision": "continue", "reason": "确认明确结果"}
        self.schema("ConfirmNextInput").validate(confirm)
        for field in ["expected_basis_fingerprint", "current_batch_id", "next_batch_id", "reason"]:
            bad = copy.deepcopy(confirm)
            del bad[field]
            self.assertFalse(self.schema("ConfirmNextInput").is_valid(bad), field)
        confirm["included_row_keys"] = ["replacement"]
        self.assertFalse(self.schema("ConfirmNextInput").is_valid(confirm))

    def test_unmet_finish_requires_note_and_exact_goal_cells(self):
        finish = {"expected_revision": "8", "expected_shortfalls": [{"platform": "baidu", "category_id": "2", "shortfall": 9}],
                  "end_reason": "unmet", "end_note": "保留原目标，资源不足结束"}
        self.schema("FinishPlanInput").validate(finish)
        finish["end_note"] = ""
        self.assertFalse(self.schema("FinishPlanInput").is_valid(finish))
        finish["end_reason"] = "cancelled"
        self.assertFalse(self.schema("FinishPlanInput").is_valid(finish))

    def test_batch_plan_projection_cannot_be_written_through_configuration(self):
        filename = "publication-batch.openapi.yaml"
        doc = self.docs[filename]
        fields = {"plan_id": "9", "plan_name": "父计划", "plan_configuration_frozen": True,
                  "plan_current_position": False, "handed_off_at": None, "action_blockers": []}
        configuration = {"platform": "baidu", "targets": [{"category_id": "2", "target_count": 1}],
                         "config": {"candidates": []},
                         "execution_setting": {"concurrency_limit": 5, "group_interval_seconds": 0,
                                               "between_group_policy": "auto_continue", "exception_task_limit": 3}}
        for path, method, command in [
            ("/api/v1/publication-batch", "post", {"name": "批次", "request_key": "create-1"}),
            ("/api/v1/publication-batch/{id}/configuration", "put", {"name": "批次", "expected_revision": "1"})]:
            validator = Draft202012Validator(
                {"$ref": f"{(API / filename).as_uri()}#/paths/{path.replace('~', '~0').replace('/', '~1')}/{method}/requestBody/content/application~1json/schema"},
                registry=self.registry)
            valid = dict(configuration, **command)
            validator.validate(valid)
            for field, value in fields.items():
                with self.subTest(method=method, field=field):
                    self.assertFalse(validator.is_valid(dict(valid, **{field: value})))
        summary = doc["components"]["schemas"]["BatchSummary"]
        for field in fields:
            self.assertIn(field, summary["properties"])
            self.assertNotIn(field, summary["required"])
            self.assertNotIn(field, doc["components"]["schemas"]["Configuration"]["properties"])
        # Use the actual response projection, including all inherited required fields.
        execution = {"phase": "not_started", "first_started_at": None, "execution_group_no": None,
                     "group_task_ids": [], "group_ended_at": None, "next_group_at": None, "pause": None,
                     "stopped_at": None, "ended_at": None, "target_count": 1, "created_count": 0,
                     "creation_shortfall": None, "published_count": 0, "target_shortfall": 1, "verification_required_count": 0, "task_counts": {}}
        value = dict(configuration, id="1", team_id="2", operator_id="3", created_by="3", revision="1",
                     name="批次", candidate_count=0, configuration_status="incomplete", execution_status="not_started",
                     execution=execution, allowed_actions=[], created_at="2026-11-25T00:00:00Z", updated_at="2026-11-25T00:00:00Z")
        value.pop("config")
        self.schema("BatchSummary", filename).validate(dict(value, **fields))
        self.schema("BatchSummary", filename).validate(value)

    def test_batch_execution_exposes_only_real_review_verification_todo(self):
        schema = self.docs["publication-batch.openapi.yaml"]["components"]["schemas"]["BatchExecution"]
        field = schema["properties"]["verification_required_count"]
        self.assertTrue(field["readOnly"])
        self.assertIn("verification_required_count", schema["required"])
        validator = Draft202012Validator(field)
        for value in [0, 1, 99]:
            validator.validate(value)
        for value in [-1, None, "1", 1.5, True]:
            self.assertFalse(validator.is_valid(value))
        self.assertIn("reviewing", field["description"])
        self.assertIn("ExceptionLimit", field["description"])

    def test_final_review_progress_waits_for_public_results_without_new_status(self):
        plan = self.plan["components"]["schemas"]["Plan"]
        progress = plan["properties"]["progress"]
        Draft202012Validator(progress).validate("awaiting_finish")
        self.assertIn("reviewing", progress["description"])
        self.assertIn("not a persisted status", progress["description"])
        self.assertNotIn("awaiting_finish", plan["properties"]["status"]["enum"])

    def test_handoff_into_cancelled_position_is_explicitly_not_a_start_receipt(self):
        value = {"plan_id": "1", "revision": "3", "previous_batch_id": "2", "current_batch_id": "3",
                 "decision": "automatic", "advanced_at": "2026-11-25T00:00:00Z",
                 "current_batch_status": "cancelled", "batch_receipt": None}
        self.schema("AdvanceResult").validate(value)
        value["current_batch_status"] = "running"
        self.assertFalse(self.schema("AdvanceResult").is_valid(value))
        value["current_batch_status"] = "cancelled"
        value["batch_receipt"] = {}
        self.assertFalse(self.schema("AdvanceResult").is_valid(value))

    def test_cross_batch_duplicate_issue_uses_two_owned_decimal_row_identities(self):
        issue = {"code": "duplicate_plan_output", "message": "同一平台成片被两批纳入",
                 "batch_id": "2", "row_key": "first", "related_batch_id": "3", "related_row_key": "second"}
        self.schema("Issue").validate(issue)
        error = {"errcode": 18003, "message": "计划候选存在跨批重复成片", "data": None, "logid": "test",
                 "error": {"code": "publication_conflict", "details": {"plan_id": "1", "current_revision": "3", "issues": [issue]}}}
        self.schema("Error").validate(error)
        for field in ["batch_id", "related_batch_id"]:
            wrong = dict(issue, **{field: 2})
            self.assertFalse(self.schema("Issue").is_valid(wrong))

    def test_record_identity_reuses_shared_opaque_audit_id(self):
        record = {"id": "audit_01jexample", "actor_id": "9", "event": "publication_plan.finish",
                  "created_at": "2026-11-25T00:00:00Z", "details": {"revision": "7"}}
        self.schema("Record").validate(record)
        record["id"] = ""
        self.assertFalse(self.schema("Record").is_valid(record))

    def test_batch_readonly_action_enum_includes_existing_plan_takeover(self):
        actions = self.docs["publication-batch.openapi.yaml"]["components"]["schemas"]["BatchSummary"]["properties"]["allowed_actions"]
        Draft202012Validator(actions).validate(["pause", "stop", "takeover"])
        self.assertFalse(Draft202012Validator(actions).is_valid(["authorize_other_user"]))

    def test_manual_failure_requires_closed_scene_and_no_publishable_work(self):
        resolve = {"attempt_id": "5", "expected_revision": "6", "conclusion": "failed",
                   "evidence": {"platform_uid": "uid", "reason": "原稿被平台明确拒发", "basis": "核实原账号与指定原稿，平台已明确拒发且浏览器现场已经关闭", "scene_closed": True, "no_publishable_work": True}}
        validator = self.schema("ResultResolution", "publication.openapi.yaml")
        validator.validate(resolve)
        for field in ["scene_closed", "no_publishable_work"]:
            bad = copy.deepcopy(resolve)
            bad["evidence"][field] = False
            self.assertFalse(validator.is_valid(bad), field)

    def test_actual_hertz_command_error_envelopes_validate(self):
        with tempfile.TemporaryDirectory(prefix="chg109-plan-error-") as directory:
            fixture = Path(directory) / "actual-errors.json"
            env = dict(os.environ, WT_MEDIA_PLAN_ERROR_FIXTURE=str(fixture))
            result = subprocess.run(["go", "test", "./internal/modules/publish", "-run",
                                     "^TestPlanNonApplicationUsesBoundActualHTTPEnvelopeAndLeavesOrdinaryConflictsUnknown$", "-count=1"],
                                    cwd=ROOT, env=env, capture_output=True, text=True, timeout=180)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            bodies = json.loads(fixture.read_text())
            self.assertEqual(len(bodies), 4)
            for body in bodies:
                self.schema("Error").validate(body)
            proof = bodies[0]["error"]["details"]
            self.schema("CommandNotApplied").validate(proof)
            self.assertFalse(proof["command_applied"])
            self.assertEqual(proof["request_key"], "original")
            for body in bodies[1:]:
                self.assertNotIn("command_applied", body["error"].get("details", {}))

    def test_non_application_receipt_is_strict_and_request_bound(self):
        details = {"command_applied": False, "command": "start", "request_key": "original",
                   "expected_revision": "2", "current_revision": "3"}
        self.schema("CommandNotApplied").validate(details)
        error = {"errcode": 1, "message": "old request", "data": None, "logid": "log",
                 "error": {"code": "publication_conflict", "details": details}}
        self.schema("Error").validate(error)
        for key, value in [("command_applied", True), ("request_key", ""), ("command", "pause"),
                           ("winning_key", "secret"), ("request_digest", "secret")]:
            bad = copy.deepcopy(error)
            bad["error"]["details"][key] = value
            self.assertFalse(self.schema("Error").is_valid(bad), key)
        for key in details:
            bad = copy.deepcopy(details)
            del bad[key]
            self.assertFalse(self.schema("CommandNotApplied").is_valid(bad), key)
        for command in ["start", "confirm-next"]:
            response = self.plan["paths"][f"/api/v1/publication-plan/{{id}}/{command}"]["post"]["responses"]["409"]
            self.assertEqual(response["content"]["application/json"]["schema"]["$ref"], "#/components/schemas/Error")

    def test_recommendation_has_typed_terminal_and_adoption_events(self):
        events = self.plan["paths"]["/api/v1/publication-plan/recommend"]["post"]["responses"]["200"]["content"]["text/event-stream"]["x-sse-events"]
        self.assertEqual(set(events), {"start", "progress", "item_result", "item_error", "done", "error"})
        self.assertIn("row_key", events["start"]["schema"]["required"])
        self.assertIn("input_revision", events["item_result"]["schema"]["required"])
        self.assertIn("logid", events["start"]["schema"]["required"])
        self.assertEqual(set(events["done"]["schema"]["required"]), {"total", "success", "failed"})
        self.schema("RecommendationResult").validate({"batches": [self.skeleton()], "resource_as_of": "2026-11-25T00:00:00Z", "issues": []})


if __name__ == "__main__":
    unittest.main()
