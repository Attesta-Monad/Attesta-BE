package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port            string
	GitHubToken     string
	AnthropicAPIKey string
	W3SToken        string
	MonadRPC        string
	DeployerKey     string
	ContractAddress string

	// AI provider: "anthropic" or "openai" (OpenAI-compatible, e.g. Groq)
	AIProvider    string
	OpenAIBaseURL string
	OpenAIAPIKey  string
	OpenAIModel   string
}

var App Config

func Load() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file, reading from environment")
	}

	App = Config{
		Port:            getEnv("PORT", "3001"),
		GitHubToken:     getEnv("GITHUB_TOKEN", ""),
		AnthropicAPIKey: getEnv("ANTHROPIC_API_KEY", ""),
		W3SToken:        getEnv("W3S_TOKEN", ""),
		MonadRPC:        getEnv("MONAD_RPC", "https://testnet-rpc.monad.xyz"),
		DeployerKey:     getEnv("DEPLOYER_PRIVATE_KEY", ""),
		ContractAddress: getEnv("CONTRACT_ADDRESS", ""),

		AIProvider:    getEnv("AI_PROVIDER", "anthropic"),
		OpenAIBaseURL: getEnv("OPENAI_BASE_URL", ""),
		OpenAIAPIKey:  getEnv("OPENAI_API_KEY", ""),
		OpenAIModel:   getEnv("OPENAI_MODEL", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
