module personal_assistant

go 1.24.0

toolchain go1.24.9

require (
	github.com/alicebob/miniredis/v2 v2.35.0
	github.com/casbin/casbin/v2 v2.11.0
	github.com/casbin/gorm-adapter/v3 v3.0.2
	github.com/cloudwego/eino v0.8.0
	github.com/cloudwego/eino-ext/components/model/ark v0.1.65
	github.com/cloudwego/eino-ext/components/model/openai v0.1.8
	github.com/cloudwego/eino-ext/components/model/qwen v0.1.8
	github.com/fvbock/endless v0.0.0-20170109170031-447134032cb6
	github.com/gin-contrib/cors v1.7.6
	github.com/gin-contrib/sessions v1.0.4
	github.com/gin-gonic/gin v1.11.0
	github.com/glebarez/sqlite v1.11.0
	github.com/go-redis/redis/v8 v8.11.5
	github.com/go-resty/resty/v2 v2.16.2
	github.com/gofrs/uuid v4.4.0+incompatible
	github.com/golang-jwt/jwt/v4 v4.5.2
	github.com/google/uuid v1.6.0
	github.com/joho/godotenv v1.5.1
	github.com/jordan-wright/email v4.0.1-0.20210109023952-943e75fe5223+incompatible
	github.com/mojocn/base64Captcha v1.3.8
	github.com/natefinch/lumberjack v2.0.0+incompatible
	github.com/qdrant/go-client v1.17.1
	github.com/qiniu/go-sdk/v7 v7.25.6
	github.com/robfig/cron/v3 v3.0.1
	github.com/songzhibin97/gkit v1.2.13
	github.com/sony/gobreaker/v2 v2.4.0
	github.com/spf13/viper v1.21.0
	github.com/urfave/cli v1.22.17
	go.uber.org/zap v1.27.0
	golang.org/x/crypto v0.48.0
	gorm.io/driver/mysql v1.6.0
	gorm.io/gorm v1.31.0
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/BurntSushi/toml v1.5.0 // indirect
	github.com/Knetic/govaluate v3.0.1-0.20171022003610-9aa49832a739+incompatible // indirect
	github.com/alex-ant/gomath v0.0.0-20160516115720-89013a210a82 // indirect
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/buger/jsonparser v1.1.1 // indirect
	github.com/bytedance/gopkg v0.1.3 // indirect
	github.com/bytedance/sonic v1.15.0 // indirect
	github.com/bytedance/sonic/loader v0.5.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudwego/base64x v0.1.6 // indirect
	github.com/cloudwego/eino-ext/libs/acl/openai v0.1.16 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/eino-contrib/jsonschema v1.0.3 // indirect
	github.com/evanphx/json-patch v0.5.2 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.9 // indirect
	github.com/gammazero/toposort v0.1.1 // indirect
	github.com/gin-contrib/sse v1.1.0 // indirect
	github.com/glebarez/go-sqlite v1.21.2 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.27.0 // indirect
	github.com/go-sql-driver/mysql v1.8.1 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/goccy/go-yaml v1.18.0 // indirect
	github.com/gofrs/flock v0.8.1 // indirect
	github.com/golang-sql/civil v0.0.0-20220223132316-b832511892a9 // indirect
	github.com/golang-sql/sqlexp v0.1.0 // indirect
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0 // indirect
	github.com/google/pprof v0.0.0-20250403155104-27863c87afa6 // indirect
	github.com/goph/emperror v0.17.2 // indirect
	github.com/gorilla/context v1.1.2 // indirect
	github.com/gorilla/securecookie v1.1.2 // indirect
	github.com/gorilla/sessions v1.4.0 // indirect
	github.com/jackc/chunkreader/v2 v2.0.1 // indirect
	github.com/jackc/pgconn v1.10.1 // indirect
	github.com/jackc/pgio v1.0.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgproto3/v2 v2.2.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.6.0 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/meguminnnnnnnnn/go-openai v0.1.2 // indirect
	github.com/microsoft/go-mssqldb v1.8.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/nikolalohinski/gonja v1.5.3 // indirect
	github.com/onsi/gomega v1.38.2 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/quic-go/qpack v0.5.1 // indirect
	github.com/quic-go/quic-go v0.54.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sagikazarmark/locafero v0.11.0 // indirect
	github.com/sirupsen/logrus v1.9.4 // indirect
	github.com/slongfield/pyfmt v0.0.0-20220222012616-ea85ff4c361f // indirect
	github.com/sourcegraph/conc v0.3.1-0.20240121214520-5f936abd7ae8 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.3.0 // indirect
	github.com/volcengine/volc-sdk-golang v1.0.23 // indirect
	github.com/volcengine/volcengine-go-sdk v1.2.9 // indirect
	github.com/wk8/go-ordered-map/v2 v2.1.8 // indirect
	github.com/yargevad/filepathx v1.0.0 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	go.uber.org/mock v0.5.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/arch v0.20.0 // indirect
	golang.org/x/exp v0.0.0-20230713183714-613f0c0eb8a1 // indirect
	golang.org/x/image v0.23.0 // indirect
	golang.org/x/mod v0.32.0 // indirect
	golang.org/x/net v0.50.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	golang.org/x/tools v0.41.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260209200024-4cfbd4190f57 // indirect
	google.golang.org/grpc v1.78.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	gorm.io/driver/postgres v1.6.0 // indirect
	gorm.io/driver/sqlserver v1.6.3 // indirect
	modernc.org/fileutil v1.0.0 // indirect
	modernc.org/libc v1.22.5 // indirect
	modernc.org/mathutil v1.5.0 // indirect
	modernc.org/memory v1.5.0 // indirect
	modernc.org/sqlite v1.23.1 // indirect
)
