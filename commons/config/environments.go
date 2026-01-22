package config

import "strings"

func IsTargetEnvFile(p string) bool {
	return strings.HasSuffix(p, ".env.json")
}

func GetEnvName(p string) string {
	return strings.TrimSuffix(p, ".env.json")
}

func MakeEnvFileName(envName string) string {
	envName = strings.TrimSuffix(envName, ".env.json")
	return envName + ".env.json"
}
