package cachekeys

const (
	// RedisKeyServerIDBySecretHash stores secret -> id in one hash.
	RedisKeyServerIDBySecretHash = "server:auth:id"
	// RedisKeyServerSecretByIDHash stores id -> secret in one hash.
	RedisKeyServerSecretByIDHash = "server:auth:secret"
	// RedisKeyServerStaticHash stores id -> static meta JSON in one hash.
	RedisKeyServerStaticHash = "server:static"
	// RedisKeyServerIDBySecretTmpHash stores rebuild temp secret -> id.
	RedisKeyServerIDBySecretTmpHash = "server:auth:id:tmp"
	// RedisKeyServerSecretByIDTmpHash stores rebuild temp id -> secret.
	RedisKeyServerSecretByIDTmpHash = "server:auth:secret:tmp"
	// RedisKeyServerStaticTmpHash stores rebuild temp id -> static meta JSON.
	RedisKeyServerStaticTmpHash = "server:static:tmp"
	// RedisKeyServerIDBySecretPrefix is the legacy per-key secret -> id prefix.
	RedisKeyServerIDBySecretPrefix = "server:auth:id:"
	// RedisKeyServerSecretByIDPrefix is the legacy per-key id -> secret prefix.
	RedisKeyServerSecretByIDPrefix = "server:auth:secret:"
	// RedisKeyServerStaticPrefix is the legacy per-key static meta prefix.
	RedisKeyServerStaticPrefix = "server:static:"
	// RedisKeyServerSecretMetaPrefix is the legacy full-meta cache keyed by secret.
	RedisKeyServerSecretMetaPrefix = "server:secret:meta:"
	// RedisKeyServerRuntimePrefix stores server runtime fields keyed by server ID.
	RedisKeyServerRuntimePrefix = "server:runtime:"
)
