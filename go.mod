module github.com/mikuspikus/go-rate-limiter

require pkg/memstorage v1.0.0
replace pkg/memstorage => ./pkg/memstorage

require pkg/rl-storage v1.0.0
replace pkg/rl-storage => ./pkg/storage

go 1.14
