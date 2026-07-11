import os

from odoo import _, api, models
from odoo.exceptions import AccessDenied


class ResUsers(models.Model):
    _inherit = "res.users"

    @api.model
    def _auth_oauth_signin(self, provider, validation, params):
        """Accept only an existing provider/subject linkage.

        auth_oauth normally attempts signup after an AccessDenied exception. This
        override is the OCA auth_oidc extension seam and deliberately does not
        call its parent, so no unmatched OIDC subject can reach that fallback.

        The pinned OCA auth_oidc (commit 2a908b498) decodes the ID token without
        verifying the ``iss`` claim, so it would honor a token minted by any
        issuer sharing the configured JWKS. When ``ODOO_OIDC_ISSUER`` is set
        (the chart always injects it on the web container), reject any token
        whose issuer differs before the identity checks run. The chart never
        sets the variable for the addon tests, so they keep exercising the
        identity path directly. The denial is intentionally generic: the login
        page must not reveal why an SSO token was refused.
        """
        expected_issuer = os.environ.get("ODOO_OIDC_ISSUER")
        if expected_issuer and validation.get("iss") != expected_issuer:
            raise AccessDenied()

        if validation.get("email_verified") is not True:
            raise AccessDenied(_("Your Zitadel email address must be verified."))

        subject = validation.get("sub")
        if not subject:
            raise AccessDenied(_("Your Zitadel identity is missing a subject."))

        oauth_user = self.sudo().search(
            [("oauth_provider_id", "=", provider), ("oauth_uid", "=", subject)],
            limit=2,
        )
        if len(oauth_user) != 1:
            raise AccessDenied(
                _("Your Zitadel identity has not been provisioned for this Odoo database.")
            )

        oauth_user.write({"oauth_access_token": params["access_token"]})
        return oauth_user.login

    @api.model
    def _configure_harpia_zitadel_provider(self):
        """Create or reconcile the sole Zitadel provider from pod metadata.

        The Helm bootstrap Job publishes this data in an Odoo-only ConfigMap;
        neither a Zitadel management credential nor a Harpia OIDC value is read
        by Odoo. ``client_secret`` is published only for confidential (web)
        clients; the pinned OCA auth_oidc sends it as HTTP Basic auth in the
        authorization-code token exchange, while PKCE-only native clients leave
        it empty.
        """
        required = (
            "ODOO_OIDC_CLIENT_ID",
            "ODOO_OIDC_AUTHORIZATION_ENDPOINT",
            "ODOO_OIDC_TOKEN_ENDPOINT",
            "ODOO_OIDC_JWKS_URI",
        )
        if any(not os.environ.get(key) for key in required):
            return

        values = {
            "name": "Zitadel",
            "client_id": os.environ["ODOO_OIDC_CLIENT_ID"],
            "client_secret": os.environ.get("ODOO_OIDC_CLIENT_SECRET") or "",
            "enabled": True,
            "body": _("Sign in with Zitadel"),
            "scope": "openid profile email",
            "flow": "id_token_code",
            "auth_endpoint": os.environ["ODOO_OIDC_AUTHORIZATION_ENDPOINT"],
            "token_endpoint": os.environ["ODOO_OIDC_TOKEN_ENDPOINT"],
            "jwks_uri": os.environ["ODOO_OIDC_JWKS_URI"],
        }
        provider = self.env["auth.oauth.provider"].sudo().search(
            [("name", "=", "Zitadel")], limit=1
        )
        if provider:
            provider.write(values)
        else:
            self.env["auth.oauth.provider"].sudo().create(values)
