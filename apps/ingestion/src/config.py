from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    DATABASE_URL: str = "postgres://vibeguard:vibeguard@localhost:5432/vibeguard"
    OPENAI_API_KEY: str = ""
    MARKER_API_KEY: str = ""
    UPLOAD_DIR: str = "/data/uploads"
    LOG_LEVEL: str = "info"

    model_config = {"env_prefix": ""}


settings = Settings()
