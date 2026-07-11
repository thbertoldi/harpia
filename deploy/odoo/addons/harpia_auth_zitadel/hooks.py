def post_init_hook(env):
    """Configure the image-owned OCA provider from non-secret pod metadata."""
    env["res.users"].sudo()._configure_harpia_zitadel_provider()
