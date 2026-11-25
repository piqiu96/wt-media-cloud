"""CHG109 provider schemas; needs pyyaml, jsonschema, openapi-spec-validator."""
from __future__ import annotations

import copy
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
                self.assertEqual(self.docs[name]["info"]["version"], "2026.11.25.4")

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

    def test_manual_failure_requires_closed_scene_and_no_publishable_work(self):
        resolve = {"attempt_id": "5", "expected_revision": "6", "conclusion": "failed",
                   "evidence": {"platform_uid": "uid", "reason": "原稿被平台明确拒发", "basis": "核实原账号与指定原稿，平台已明确拒发且浏览器现场已经关闭", "scene_closed": True, "no_publishable_work": True}}
        validator = self.schema("ResultResolution", "publication.openapi.yaml")
        validator.validate(resolve)
        for field in ["scene_closed", "no_publishable_work"]:
            bad = copy.deepcopy(resolve)
            bad["evidence"][field] = False
            self.assertFalse(validator.is_valid(bad), field)

    def test_recommendation_has_typed_terminal_and_adoption_events(self):
        events = self.plan["paths"]["/api/v1/publication-plan/recommend"]["post"]["responses"]["200"]["content"]["text/event-stream"]["x-sse-events"]
        self.assertEqual(set(events), {"start", "progress", "item_result", "item_error", "done", "error"})
        self.schema("RecommendationResult").validate({"batches": [self.skeleton()], "resource_as_of": "2026-11-25T00:00:00Z", "issues": []})


if __name__ == "__main__":
    unittest.main()
