module github.com/openmvcore

go 1.21

require (
	github.com/wmnsk/go-gtp v0.8.0
	github.com/free5gc/go-upf v0.0.0-20230801080000-000000000000 // indirect, will be replaced with our fork
	github.com/redis/go-redis/v9 v9.5.1
	github.com/lib/pq v1.10.9
	github.com/gin-gonic/gin v1.9.1
	github.com/spf13/viper v1.18.2
	github.com/go-playground/validator/v10 v10.19.0
	github.com/google/uuid v1.6.0
	github.com/nats-io/nats.go v1.33.1
	google.golang.org/grpc v1.62.1
	google.golang.org/protobuf v1.33.0
)

// TODO: Add go-amf dependency once available
// TODO: Replace go-upf with our fork once created 