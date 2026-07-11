from odoo.exceptions import AccessDenied
from odoo.tests.common import TransactionCase


class TestHarpiaAuthZitadel(TransactionCase):
    @classmethod
    def setUpClass(cls):
        super().setUpClass()
        cls.provider = cls.env["auth.oauth.provider"].create(
            {
                "name": "Zitadel test provider",
                "client_id": "odoo-test-client",
                "enabled": True,
                "body": "Sign in with Zitadel",
                "flow": "id_token_code",
                "auth_endpoint": "https://zitadel.test/oauth/v2/authorize",
                "token_endpoint": "https://zitadel.test/oauth/v2/token",
                "jwks_uri": "https://zitadel.test/oauth/v2/keys",
            }
        )

    def _signin(self, subject, email_verified=True):
        return self.env["res.users"]._auth_oauth_signin(
            self.provider.id,
            {"sub": subject, "email_verified": email_verified},
            {"access_token": "test-access-token"},
        )

    def test_linked_verified_identity_is_accepted(self):
        user = self.env["res.users"].create(
            {
                "name": "Linked user",
                "login": "linked@example.test",
                "oauth_provider_id": self.provider.id,
                "oauth_uid": "zitadel-linked-subject",
            }
        )

        self.assertEqual(self._signin("zitadel-linked-subject"), user.login)

    def test_unknown_subject_is_rejected_without_creating_user_or_link(self):
        before = self.env["res.users"].search_count([])

        with self.assertRaises(AccessDenied):
            self._signin("unknown-zitadel-subject")

        self.assertEqual(self.env["res.users"].search_count([]), before)
        self.assertFalse(
            self.env["res.users"].search(
                [("oauth_provider_id", "=", self.provider.id), ("oauth_uid", "=", "unknown-zitadel-subject")]
            )
        )

    def test_missing_or_false_email_verified_is_rejected(self):
        user = self.env["res.users"].create(
            {
                "name": "Linked user",
                "login": "verified@example.test",
                "oauth_provider_id": self.provider.id,
                "oauth_uid": "verified-subject",
            }
        )
        for claim in (False, None):
            with self.subTest(email_verified=claim), self.assertRaises(AccessDenied):
                validation = {"sub": user.oauth_uid}
                if claim is not None:
                    validation["email_verified"] = claim
                self.env["res.users"]._auth_oauth_signin(
                    self.provider.id, validation, {"access_token": "test-access-token"}
                )

    def test_unverified_email_match_is_not_an_identity_fallback(self):
        self.env["res.users"].create(
            {"name": "Email-only user", "login": "same@example.test", "email": "same@example.test"}
        )
        before = self.env["res.users"].search_count([])

        with self.assertRaises(AccessDenied):
            self.env["res.users"]._auth_oauth_signin(
                self.provider.id,
                {"sub": "different-subject", "email": "same@example.test", "email_verified": False},
                {"access_token": "test-access-token"},
            )

        self.assertEqual(self.env["res.users"].search_count([]), before)
