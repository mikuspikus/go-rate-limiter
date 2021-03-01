module redis-storage

require (
	github.com/gomodule/redigo v1.8.4
	pkg/rl-storage v1.0.0
)

replace pkg/rl-storage => ./../storage

go 1.14
