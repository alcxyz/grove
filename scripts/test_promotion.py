import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("promotion", Path(__file__).with_name("check-promotion.py"))
promotion = importlib.util.module_from_spec(spec)
spec.loader.exec_module(promotion)


def event(head="dev", repo="alcxyz/grove", base="main"):
    return {"pull_request": {"head": {"ref": head, "repo": {"full_name": repo}},
                             "base": {"ref": base}}}


class PromotionTests(unittest.TestCase):
    def test_new_version_from_dev_is_accepted(self):
        promotion.check(event(), "0.10.0", "0.9.2", False)

    def test_unreleased_or_reused_versions_are_rejected(self):
        for current, base, tag in [("0.9.2", "0.9.2", False),
                                   ("0.9.1", "0.9.2", False),
                                   ("0.10.0", "0.9.2", True)]:
            with self.assertRaises(ValueError):
                promotion.check(event(), current, base, tag)

    def test_main_rejects_feature_branches_and_fork_dev(self):
        for pr in [event(head="feature"), event(repo="someone/grove")]:
            with self.assertRaises(ValueError):
                promotion.check(pr, "0.10.0", "0.9.2", False)

    def test_development_prs_do_not_require_a_release(self):
        promotion.check(event(head="feature", base="dev"), "0.9.2-dev", "0.9.2", True)

    def test_versions_are_compared_numerically(self):
        promotion.check(event(), "0.10.0", "0.9.2", False)
        for value in ["v1.0.0", "01.0.0", "1.0.0-rc1"]:
            with self.assertRaises(ValueError):
                promotion.semver(value)
