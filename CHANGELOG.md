# Changelog

## [1.1.0](https://github.com/pay-theory/lift/compare/v1.1.0...v1.1.0) (2026-01-19)


### ⚠ BREAKING CHANGES

* EnableVPCAuthorizer signature changed from (partner, stage string) to (authorizerFunctionArn, authorizerName, authorizerCredentialsArn string)

### Features

* Add `build` command to compile Lambda functions with configurable build settings in `lift.yaml`. ([d7212af](https://github.com/pay-theory/lift/commit/d7212affef9dbfe8324715e63d09b55709e789a7))
* Add `PathRoutedFrontendDistribution` construct and update default CloudFront API cache and origin request policies. ([04a8e57](https://github.com/pay-theory/lift/commit/04a8e572e25002535fb70083d0c993d62747c7e0))
* add api_key_id sanitization with alphanumeric partial masking ([964f77a](https://github.com/pay-theory/lift/commit/964f77ac144334853c485006026180a38dc23fc4))
* add api_key_id sanitization with alphanumeric partial masking ([95f7ed0](https://github.com/pay-theory/lift/commit/95f7ed0b38ba110e3125c812489fac9e00b99743))
* Add AWS error handling utilities to observability toolkit ([9e0bb2b](https://github.com/pay-theory/lift/commit/9e0bb2bf7b96a65fe39a7ea4d7e00405330c0956))
* add CloudWatch alarm constructs for SQS, API Gateway, and DynamoDB ([6e4cd7c](https://github.com/pay-theory/lift/commit/6e4cd7cfa9687864d9994f3286f2601b5cc9941d))
* add CloudWatch alarm constructs for SQS, API Gateway, and DynamoDB ([a28b61c](https://github.com/pay-theory/lift/commit/a28b61c6cde1af8274cd5d1dba4d44a8fdf1e158))
* Add comprehensive field classification to dataprotection ([ba5ee48](https://github.com/pay-theory/lift/commit/ba5ee484097198acac3b78098871f131798ebe02))
* Add comprehensive field classification to dataprotection ([a9695f7](https://github.com/pay-theory/lift/commit/a9695f7172d2887fef948f64dfacc00802834c6c))
* Add comprehensive field classification to dataprotection ([30bcd29](https://github.com/pay-theory/lift/commit/30bcd299ed800bd15ada6f9772b04fc8a82bcc3f))
* Add comprehensive idempotency middleware documentation and examples ([c0befe7](https://github.com/pay-theory/lift/commit/c0befe78561fff7712b5d386bb7d8ede37b2b32f))
* Add comprehensive README to basic-api template, streamline Go module handling in CI/CD and CDK, and remove `constructs-go/constructs/v10` dependency. ([17cb015](https://github.com/pay-theory/lift/commit/17cb0154019ae693f6b10cf7c76fe6a06d6e1b47))
* add configurable CORS origins to API construct ([bfac375](https://github.com/pay-theory/lift/commit/bfac375c8ead103b1c19c0a64b7b8e4b65fb11f8))
* add configurable VPC endpoints to enhanced security ([d95665b](https://github.com/pay-theory/lift/commit/d95665bd945c1832b5391d9ef0467c11ff5a0eae))
* add domain management constructs for API Gateway ([02db1a3](https://github.com/pay-theory/lift/commit/02db1a310089d3b975a954e5f1b8b4c094524261))
* add event source mapping construct with cross-region support ([ada7432](https://github.com/pay-theory/lift/commit/ada7432291d80cb8236d2521375c017e1ac0e8a4))
* Add extensive test coverage across features, constructs, and CLI, and introduce a new DynamoDB facade. ([221d5b5](https://github.com/pay-theory/lift/commit/221d5b578f3b26bdd55d13e555e2b69981df3863))
* Add IP authorization security feature with AWS SSM integration ([22ce388](https://github.com/pay-theory/lift/commit/22ce388c9da58b9d00642cc9a70bc186eba7c76a))
* Add IP authorization security feature with AWS SSM integration ([5987ab4](https://github.com/pay-theory/lift/commit/5987ab49c0026ff7b028ff4d51a751622c68fbd4))
* Add IsLambda() helper method to App structure ([e7bcaac](https://github.com/pay-theory/lift/commit/e7bcaacfe48a5e750c6372667fb9236383e97db7))
* add Lambda CloudWatch alarms construct ([1a0e942](https://github.com/pay-theory/lift/commit/1a0e94270df754776d5738c5e7a675dc832637ec))
* add Lambda CloudWatch alarms construct ([ec71358](https://github.com/pay-theory/lift/commit/ec713580114d6b8095f341be5e835dba956a6ae8))
* add Lambda IAM role construct with comprehensive permissions ([21b73a9](https://github.com/pay-theory/lift/commit/21b73a93890e925c2dc043a9fdb9238148e7e821))
* Add LiftRestAPI construct for API Gateway v1 and implement response streaming (SSE) support with accompanying documentation. ([cd69f6b](https://github.com/pay-theory/lift/commit/cd69f6bce1f466acc5e1d75fc519dbf112508ade))
* Add LoggerConfig helper functions with configurable log level ([7a15a1d](https://github.com/pay-theory/lift/commit/7a15a1d9eebd72d8f88eca47a2531699093b9606))
* add multi-region KMS key construct with HMAC support ([d0e3849](https://github.com/pay-theory/lift/commit/d0e38493255e495665dcf33350d67bbe1a98e15e))
* Add mutex protection to EventRouter for thread safety ([57532e0](https://github.com/pay-theory/lift/commit/57532e00ffa8814f0fa1785f79665d2cb1b3fb70))
* Add partner and target mode support to `up` and `down` commands, introduce new `basic-api-pt` project templates, and enforce partner flag usage. ([47bc7b0](https://github.com/pay-theory/lift/commit/47bc7b08256bc058fde24bfcd307fbc21b3d8927))
* Add per-method streaming overrides to `LiftRestAPI` integration… ([3cbeca4](https://github.com/pay-theory/lift/commit/3cbeca42a28d15df453481094b584ce94bb0cdf0))
* Add per-method streaming overrides to `LiftRestAPI` integrations and update `SSEResponse` for API Gateway v1 streaming. ([796689f](https://github.com/pay-theory/lift/commit/796689fb8e9101974d52ea33000ad786f9274a92))
* Add prerequisite checks to CLI commands, new getting started and CI documentation, and a release workflow. ([4dab741](https://github.com/pay-theory/lift/commit/4dab7411d730618aed440951e0800c6afb32d398))
* Add response interception support for middleware ([a0a3f14](https://github.com/pay-theory/lift/commit/a0a3f14f7ca817b6ebc7f76cfc4ca764bebf88f8))
* Add response interception support for middleware ([c2a7c35](https://github.com/pay-theory/lift/commit/c2a7c35b93a6d045a17fba35741af54653b09def))
* add REST API support and refactor common API utilities ([13c4972](https://github.com/pay-theory/lift/commit/13c4972f671b8f006a3bdb7bbb6b075c9430f657))
* add REST API support and refactor common API utilities ([7aecad1](https://github.com/pay-theory/lift/commit/7aecad126dd1cf400575d997e30a5cbd04fdbf52))
* Add RunLocalTest method for local event testing ([007a884](https://github.com/pay-theory/lift/commit/007a88465bb8e4895f3da3f049cb66a248c254e5))
* Add RunLocalTest method for local event testing ([7e88942](https://github.com/pay-theory/lift/commit/7e8894219360dcd39c601053ccb645a95e71d760))
* Add S3 object key pattern matching support ([774b328](https://github.com/pay-theory/lift/commit/774b3286b0948617521d2b3290c8fafb0dd5ca2e))
* add S3 object store for large SQS payloads and make SHA256 verification option nullable in envelope. ([7849962](https://github.com/pay-theory/lift/commit/7849962242bcc607ebb596cb0e417b862afe606a))
* Add SNS error notification support to CloudWatch logger ([a372d77](https://github.com/pay-theory/lift/commit/a372d777f32a8ab43b440b08ae8f3bb7bb0d576c))
* Add SNS processor template and update CDK best practices documentation ([f0bff79](https://github.com/pay-theory/lift/commit/f0bff79e4911cde2a26cdf04e192c4def04df12f))
* add SQS large payload support via S3 offloading and integrate into SQSProcessor construct. ([d2c3215](https://github.com/pay-theory/lift/commit/d2c321505518b757ccd28bbc1b3eb86503084f2f))
* Add SQS large payload support with S3 bucket, lifecycle, and IAM permissions. ([57f090e](https://github.com/pay-theory/lift/commit/57f090eb2da3e7f87af9866ce83b0d21d029c7ed))
* add SQS queue construct with comprehensive configuration ([3d31e29](https://github.com/pay-theory/lift/commit/3d31e2955d88f4ce3614c3bb604983f4410add23))
* Add support for AWS SQS Extended Client large payload parsing and processing. ([1fb00fe](https://github.com/pay-theory/lift/commit/1fb00fe5603d34f1ca0a699d695fa69ba07e3f2a))
* add WebSocket action routing, introduce naming utilities, and enhance EventBus with DynamoDB durability. ([79d3e22](https://github.com/pay-theory/lift/commit/79d3e2291e6e92eb0743016149d9717ee190f492))
* Add working rate limiting middleware using limited library ([d45a43d](https://github.com/pay-theory/lift/commit/d45a43de2228848f3a70bf5051c022ae94041ad5))
* Add working rate limiting middleware using limited library ([b290c07](https://github.com/pay-theory/lift/commit/b290c071eb426ab1d540be774bf3750af0fee013))
* comprehensive security fixes and framework improvements ([813d693](https://github.com/pay-theory/lift/commit/813d693a3013ae06e6740c0a94e1757df6a12d99))
* Enhance API Gateway REST API (v1) streaming with `multiValueHeaders` support, per-method timeout overrides, and documentation for throughput limits and heartbeats. ([86f7f84](https://github.com/pay-theory/lift/commit/86f7f845903069b92cec072585f23af73f99510e))
* enhance DynamoDB construct with GSI, encryption, and auto-scaling ([0eb9e23](https://github.com/pay-theory/lift/commit/0eb9e2342dfb83a686899c7a8fedc2ccbed60802))
* Enhance LiftError Error() method with cause and details ([b7b051b](https://github.com/pay-theory/lift/commit/b7b051bc2ab45b8b6e919772af68b635f7c7ce0b))
* implement event routing fix for non-HTTP Lambda events ([6f81d40](https://github.com/pay-theory/lift/commit/6f81d409b9339fc0d13212ff7edc72ea531b8e11))
* Implement SQS batch processor to manage large payloads stored in S3, supporting hydration, message processing, and object deletion. ([3de39db](https://github.com/pay-theory/lift/commit/3de39db4ec65450391aa06c3f72f71fd92136a01))
* Improve sanitization with blocklist approach and nested JSON ha… ([f59df09](https://github.com/pay-theory/lift/commit/f59df090a2986ca68c7de8306d31142e9530f880))
* Improve sanitization with blocklist approach and nested JSON handling ([0257a6c](https://github.com/pay-theory/lift/commit/0257a6c1e8be102bd41548d2a30d76e5cec07f51))
* introduce `add` command to CLI and add dynamic function placeholders to project templates. ([edf73be](https://github.com/pay-theory/lift/commit/edf73beba2a017d7cbe9b5ad9be1ec919d15efc1))
* Introduce `down` command for infrastructure teardown, alongside new state management and domain resolution capabilities. ([c3a9736](https://github.com/pay-theory/lift/commit/c3a97367c163498395431729a2623268703aafc9))
* Introduce CloudFront helper functions and enhance event bus components. ([4ff9c21](https://github.com/pay-theory/lift/commit/4ff9c2153fd6304af372d97ff9369b83bfb197a3))
* Introduce comprehensive event bus streaming, scheduling, and processing capabilities with new CDK constructs and documentation, alongside updates to idempotency middleware and CDN patterns. ([d24b396](https://github.com/pay-theory/lift/commit/d24b396914e9032819a15fefcb4ad387feb48e8c))
* Introduce DynamoDB stream event handling with a dedicated registration method and context record parsing. ([1d29d61](https://github.com/pay-theory/lift/commit/1d29d6113c55d06c38b05fc3ddc431f4517e82d4))
* introduce lift CLI with new and up commands to bootstrap projects and manage deployments. ([8a622dd](https://github.com/pay-theory/lift/commit/8a622dd03dce9be4478dd2731284729b297bd2d0))
* Introduce new project templates for event-driven, microservice, and merchant applications, and update CLI commands and embedding for template management. ([a4fcc84](https://github.com/pay-theory/lift/commit/a4fcc8419e8863dfeaf7b9e3c95a60e1eb426ed1))
* introduce SQS large payload support via S3 offloading with new documentation and an example. ([adaad7f](https://github.com/pay-theory/lift/commit/adaad7f923468b7d52e842b09c6a6a2b53ed4979))
* optimize struct field alignment to reduce memory usage ([f117034](https://github.com/pay-theory/lift/commit/f117034e63aa98cece9da39d4fe54fa2ee728aef))
* prevent IP address redaction in data protection ([9ba8d97](https://github.com/pay-theory/lift/commit/9ba8d97d63d45b20bf4e47239be19d737f0176f3))
* prevent key fields from being redacted in logs ([a92cd6e](https://github.com/pay-theory/lift/commit/a92cd6ee6640877f064e91a72041b8ade995cc31))
* prevent key fields from being redacted in logs ([d63e40e](https://github.com/pay-theory/lift/commit/d63e40edd848c3bc1658e4aa9e913d556bbd2d40))
* Return raw SQS batch responses and propagate SQS handler errors for SQS triggers. ([4e65ed7](https://github.com/pay-theory/lift/commit/4e65ed7410f9c4f215b92990a4b33f32f48d25c9))
* **sanitization:** add cardholder and cardholder_name to fully redacted PII fields ([0d34564](https://github.com/pay-theory/lift/commit/0d34564ba1d67947436e42bd5160a8f47e47607a))
* Show BIN (first 6 digits) + last 4 in card sanitization ([c42b9ca](https://github.com/pay-theory/lift/commit/c42b9ca724949b1269624240c1e4e3c98ac5eec0))
* universal SNS error notifications for all Pay Theory services ([7fee7fd](https://github.com/pay-theory/lift/commit/7fee7fd86aa6a988c2a86c0ccfdb8eb1f238bf88))
* universal SNS error notifications for all Pay Theory services ([c08cf43](https://github.com/pay-theory/lift/commit/c08cf4310dd22856e173f42f8cfebff65d66427b))


### Bug Fixes

* add austin partner to qakernel routing ([5d0a554](https://github.com/pay-theory/lift/commit/5d0a55462060b5da45da77ce95aa79b29d901b04))
* add AWS region detection to Limited rate limit helper functions ([4bc0e0a](https://github.com/pay-theory/lift/commit/4bc0e0ab00cff6aebacd12fafee7186fc0b377ee))
* add AWS region detection to Limited rate limit helper functions ([9c6dbed](https://github.com/pay-theory/lift/commit/9c6dbed5e090d9cec5a16c255e8592f1cd17ff21))
* add error handling for 108 critical errcheck violations ([235d5ac](https://github.com/pay-theory/lift/commit/235d5aceb6dd0ee6a67b73916341928a571cc149))
* add missing dist directory for CDK construct tests ([97c6148](https://github.com/pay-theory/lift/commit/97c6148b4fa35c55a0a98f05c238427ab716453c))
* add proper error handling for critical operations ([cc55fa2](https://github.com/pay-theory/lift/commit/cc55fa2cf82faebf82fa83bc9f7447d2609b7d02))
* align Lift CDK tables with DynamORM field name conventions ([3a3911b](https://github.com/pay-theory/lift/commit/3a3911be73b3a81b8b858fb922a1cbe599db54e0))
* apply content-based deduplication to FIFO dead-letter queues ([89dcfcb](https://github.com/pay-theory/lift/commit/89dcfcb789d3e77ab596df080f1674781ca0d862))
* apply content-based deduplication to FIFO dead-letter queues ([60c9306](https://github.com/pay-theory/lift/commit/60c9306e2f32ea78b271f5c8cca1066e5a866df2))
* **ci:** align release line to v1.1.0 ([453b13d](https://github.com/pay-theory/lift/commit/453b13d2b188625d1d3856e2a9cae78e5901f3f7))
* **ci:** publish release assets via github.token ([996a3bd](https://github.com/pay-theory/lift/commit/996a3bd1bb5d15bbeb510db608c62e7968de0071))
* **ci:** publish release assets via github.token ([084f079](https://github.com/pay-theory/lift/commit/084f0796bca751000618cdd1574a06dd15c9c7d5))
* **ci:** support immutable GitHub releases ([01c6855](https://github.com/pay-theory/lift/commit/01c68559f9b045816ef82d245b19319d3385dac8))
* **ci:** support immutable GitHub releases ([d2fb7ca](https://github.com/pay-theory/lift/commit/d2fb7ca9453f8acbdf2c60f5825e611f0fae5e25))
* **cli:** mock CDK/Go presence in tests to support CI environments without them ([fb66ad2](https://github.com/pay-theory/lift/commit/fb66ad28b20bb256a499a5ef2cf1be6eeb9c62a7))
* comprehensive error handling for all errcheck violations ([c38dca9](https://github.com/pay-theory/lift/commit/c38dca98cc3945da53f3481f50291a7b33d1ca27))
* Convert partner and stage to lowercase in SNS notifications ([1bea7fa](https://github.com/pay-theory/lift/commit/1bea7faceb399cffd9bb69538a8ddbbd541509e7))
* correct stage prefix stripping to avoid false matches ([d440086](https://github.com/pay-theory/lift/commit/d440086209d5a4a5e03598de3e47a8c1931fa922))
* Exclude card_bin from log redaction as it's not sensitive data ([ca71e64](https://github.com/pay-theory/lift/commit/ca71e642d77300f5fa4eab113ebc9ab1e307556d))
* Fix idempotency middleware test failures ([0c1c311](https://github.com/pay-theory/lift/commit/0c1c311e56f0115e59f341264b07f82ff375db26))
* Fix idempotency middleware test failures ([833eeb1](https://github.com/pay-theory/lift/commit/833eeb13ea4acf9d19d8c2a8f0c65abe321effed))
* Fix WebSocket routing in HandleRequest method ([f84a883](https://github.com/pay-theory/lift/commit/f84a8839c35526ca4e187c1cd3f21d10065168f0))
* Fix WebSocket routing in HandleRequest method ([18d4d25](https://github.com/pay-theory/lift/commit/18d4d25a029ecac823a5cb9b985b0649753d69af))
* Handle API Gateway v2 stage prefix in paths for custom domains ([c210756](https://github.com/pay-theory/lift/commit/c210756172eaf3831bfd616cb9834948b7273f44))
* improve API Gateway adapter detection and error handling ([de89abc](https://github.com/pay-theory/lift/commit/de89abcf2b6479692e1522ec7358c941bae0e3a4))
* improve API Gateway adapter detection and error handling ([d8c1885](https://github.com/pay-theory/lift/commit/d8c1885995a393fde51634f2aaa4edde2436d8d1))
* improve API Gateway stage management and domain mapping ([af18cae](https://github.com/pay-theory/lift/commit/af18caefb39cd2ee9a48da73dd74122746fb852d))
* improve API Gateway stage management and domain mapping ([28c0ac7](https://github.com/pay-theory/lift/commit/28c0ac764c2e962f00518328305e7bc42607c5d3))
* Improve sensitive data redaction in CloudWatch logger ([6961d1b](https://github.com/pay-theory/lift/commit/6961d1b700faf1ab80d17b4367e9a9542b2afca8))
* Initialize real logger in Logger middleware when ctx.Logger is nil ([5f2161d](https://github.com/pay-theory/lift/commit/5f2161dc2fd2aa2a24fe124bd35c61982813565e))
* Initialize real logger in Logger middleware when ctx.Logger is nil ([d222ed0](https://github.com/pay-theory/lift/commit/d222ed0e225be0ce7bcd8c68ac92d78599f5bbc3))
* install proper gosec version and fix golangci-lint config ([6033af5](https://github.com/pay-theory/lift/commit/6033af545246d5d995f9f902b1133443ac7f9af2))
* Middleware not executing in Lambda HandleRequest + Example compilation fixes ([51c408c](https://github.com/pay-theory/lift/commit/51c408c39ff939bc83890cdc8e9bf6869e763c93))
* minimize WebSocket CDK resource IDs to prevent CloudFormation errors ([1fbabf2](https://github.com/pay-theory/lift/commit/1fbabf2fdf56ed72394ff103483a623648fff1c3))
* minimize WebSocket CDK resource IDs to prevent CloudFormation name length errors ([7122fe5](https://github.com/pay-theory/lift/commit/7122fe5f65b0499a3e89db931b7e59f0a7cc5242))
* optimize gosec scope to prevent timeouts ([433715e](https://github.com/pay-theory/lift/commit/433715e9eddc081f0d81ed6f95e206c0feb3e28e))
* optimize struct field alignment to reduce memory usage ([e86ce95](https://github.com/pay-theory/lift/commit/e86ce95d58b6b851af70bc29872dcb536e3c9e0f))
* prevent ID fields from being redacted in logs ([114acb4](https://github.com/pay-theory/lift/commit/114acb4b2065a4f0cd6b547d0efb89b9ccaa9a8b))
* prevent ID fields from being redacted in logs ([a9d6473](https://github.com/pay-theory/lift/commit/a9d6473a2a0ba07ad8ceefe849837818d570e211))
* properly fix WebSocket CDK resource name length issue ([3ca78ee](https://github.com/pay-theory/lift/commit/3ca78ee81c6f026a7a3929fb217c516aaf431a88))
* redesign MultiTenantAPI to use DynamORM-compatible tables ([0db11f9](https://github.com/pay-theory/lift/commit/0db11f9f7e571ba7c18b0960ab390aa6393b34d8))
* remove all log-related parameters from Lambda constructs ([ce3c8de](https://github.com/pay-theory/lift/commit/ce3c8dee4e4eef6e5d93e36493a41252e5ea9f3a))
* remove all log-related parameters from Lambda constructs ([b3bbf63](https://github.com/pay-theory/lift/commit/b3bbf63944dfc0bfa4fddcc85ba975dfde9c19ed))
* remove all log-related parameters from Lambda constructs and fix mutex issues ([d41a1b8](https://github.com/pay-theory/lift/commit/d41a1b86b3fa16e7da965a84596973a69028efa7))
* remove broken MultiTenantAPI construct ([2c5ab4d](https://github.com/pay-theory/lift/commit/2c5ab4dc72023df5dc48631ce60aa25009ba676e))
* remove test file with non-existent dynamorm/v2 import ([e1c38fd](https://github.com/pay-theory/lift/commit/e1c38fdf07b1d449e6cdd723c9184571502bb668))
* replace deprecated gofmt linter with gofumpt ([ca230ee](https://github.com/pay-theory/lift/commit/ca230ee4daffc50d2ff81063a4ab47bfeebc1e18))
* resolve 22 staticcheck violations for correctness and performance ([4f825af](https://github.com/pay-theory/lift/commit/4f825afd94537081ed9c9d8b7b7b221bdfc4bf82))
* resolve 442 govet issues including critical shadow variables ([b85e2f5](https://github.com/pay-theory/lift/commit/b85e2f510fc146bf35d4b61726e2b3de7bcd087e))
* resolve 75 critical errcheck violations in production code ([a0b36cc](https://github.com/pay-theory/lift/commit/a0b36ccef39b9d050668e636d6e48619e6df5691))
* resolve 89 critical errcheck violations in production paths ([db47eeb](https://github.com/pay-theory/lift/commit/db47eeb0735fbe9a0c2c61ede9e595291962a8c6))
* resolve additional lint violations for code quality ([cb251fb](https://github.com/pay-theory/lift/commit/cb251fb0e52bffa1e25787dddfc30313e5b9ddc5))
* resolve all 42 gosec security violations ([b1fff0f](https://github.com/pay-theory/lift/commit/b1fff0fe2a7208f4fb9c2fc308202324b1a732d0))
* resolve all failing tests and complete DynamORM table standardization ([c4990cd](https://github.com/pay-theory/lift/commit/c4990cd649defbe26aad28fba0572d9b49b705a0))
* resolve all failing tests and complete DynamORM table standardization ([7553aa1](https://github.com/pay-theory/lift/commit/7553aa1b49504fd9986103519f690c4212a7585e))
* resolve compilation error in debug logging ([9955509](https://github.com/pay-theory/lift/commit/99555091e495e328ca7f19117d30d9c40d0aeceb))
* resolve critical govet correctness issues ([70be218](https://github.com/pay-theory/lift/commit/70be2181f23c8366cf87e6571dcdd1c0a6cbf5f9))
* resolve critical govet issues and optimize struct alignment ([5822ed2](https://github.com/pay-theory/lift/commit/5822ed28ca86d62b541fcbd28d86a43476620fa5))
* resolve critical govet shadow variable and fieldalignment violations ([d9cfa9f](https://github.com/pay-theory/lift/commit/d9cfa9fa005a6b04fa379c598f793bb01374d86c))
* resolve critical govet shadow variable and optimize field alignment ([87b5ca7](https://github.com/pay-theory/lift/commit/87b5ca7b54619e8edb620d05bf06fb70158f33ce))
* resolve critical govet shadow variable violations ([96830e3](https://github.com/pay-theory/lift/commit/96830e3dbb43e53527fdae9d723bbe172bef8fb2))
* resolve critical lint violations for production reliability ([bf1b9b8](https://github.com/pay-theory/lift/commit/bf1b9b8fde0a41fc38edeb0836ac00ad58b301a6))
* Resolve duplicate CloudWatch logs during logger shutdown ([28ff5e8](https://github.com/pay-theory/lift/commit/28ff5e8a831248426b7a3cbc9e4bb5eb975b2c68))
* Resolve LiftError status codes not being properly mapped to HTTP responses ([fa0faac](https://github.com/pay-theory/lift/commit/fa0faaca7996c75fb9ac12a2a218bb949740f3a1))
* Resolve LiftError status codes not being properly mapped to HTTP responses ([79279d2](https://github.com/pay-theory/lift/commit/79279d26af03f6a9aa78ba4406b084a275e0ff5a))
* resolve LogGroup duplication issue in LiftFunction construct ([0764965](https://github.com/pay-theory/lift/commit/076496550d53427b79a4f0a1a057ff053c236b0b))
* resolve LogGroup duplication issue in LiftFunction construct ([df5da1a](https://github.com/pay-theory/lift/commit/df5da1a2ec5d98556d9690928b228799d029ff2b))
* resolve remaining critical govet issues ([6e3c993](https://github.com/pay-theory/lift/commit/6e3c993e9e249b71caf8930b650add6b51eade0c))
* resolve remaining lint violations and reduce from 3000+ to 120 issues ([029d10c](https://github.com/pay-theory/lift/commit/029d10c0979b835a501d9d807fbb281e21e1c006))
* resolve test failures across multiple packages ([7df2b63](https://github.com/pay-theory/lift/commit/7df2b63f7df76c20746d9d43559e9753371262fc))
* resolve test failures across multiple packages ([edd1127](https://github.com/pay-theory/lift/commit/edd1127d25d1d572fe27aa68742f4224cd5df653))
* update CDK construct tests to use DynamORM field naming standards ([f878e85](https://github.com/pay-theory/lift/commit/f878e85be1af7126e70eb4e6eedabfd11d451388))
* update CDK construct tests to use DynamORM field naming standards ([ac1ee45](https://github.com/pay-theory/lift/commit/ac1ee458486639ec6d234b9f3bde88d7679d9a01))
* update test to expect standard pk naming for multi-tenant tables ([802a798](https://github.com/pay-theory/lift/commit/802a7988a3f6eecffd1075a6268af12e215d5f08))
* use RemovalPolicy_DESTROY instead of non-existent RemovalPolicy_DELETE ([f56175a](https://github.com/pay-theory/lift/commit/f56175a2981b3b1dfca85206cf8cd68762ad6b01))
* use standard pk/sk naming for DynamoDB multi-tenant tables ([802df1f](https://github.com/pay-theory/lift/commit/802df1f7c11e6e597faa283fd4bbaed1b0dd7e2f))


### Code Refactoring

* make VPC authorizer generic by accepting ARNs directly ([fcc7968](https://github.com/pay-theory/lift/commit/fcc7968a8563e1bf6ff68cfd36db5e5313556da7))

## [1.1.0](https://github.com/pay-theory/lift/compare/v1.1.0...v1.1.0) (2026-01-14)


### ⚠ BREAKING CHANGES

* EnableVPCAuthorizer signature changed from (partner, stage string) to (authorizerFunctionArn, authorizerName, authorizerCredentialsArn string)

### Features

* Add `build` command to compile Lambda functions with configurable build settings in `lift.yaml`. ([d7212af](https://github.com/pay-theory/lift/commit/d7212affef9dbfe8324715e63d09b55709e789a7))
* Add `PathRoutedFrontendDistribution` construct and update default CloudFront API cache and origin request policies. ([04a8e57](https://github.com/pay-theory/lift/commit/04a8e572e25002535fb70083d0c993d62747c7e0))
* add api_key_id sanitization with alphanumeric partial masking ([964f77a](https://github.com/pay-theory/lift/commit/964f77ac144334853c485006026180a38dc23fc4))
* add api_key_id sanitization with alphanumeric partial masking ([95f7ed0](https://github.com/pay-theory/lift/commit/95f7ed0b38ba110e3125c812489fac9e00b99743))
* Add AWS error handling utilities to observability toolkit ([9e0bb2b](https://github.com/pay-theory/lift/commit/9e0bb2bf7b96a65fe39a7ea4d7e00405330c0956))
* add CloudWatch alarm constructs for SQS, API Gateway, and DynamoDB ([6e4cd7c](https://github.com/pay-theory/lift/commit/6e4cd7cfa9687864d9994f3286f2601b5cc9941d))
* add CloudWatch alarm constructs for SQS, API Gateway, and DynamoDB ([a28b61c](https://github.com/pay-theory/lift/commit/a28b61c6cde1af8274cd5d1dba4d44a8fdf1e158))
* Add comprehensive field classification to dataprotection ([ba5ee48](https://github.com/pay-theory/lift/commit/ba5ee484097198acac3b78098871f131798ebe02))
* Add comprehensive field classification to dataprotection ([a9695f7](https://github.com/pay-theory/lift/commit/a9695f7172d2887fef948f64dfacc00802834c6c))
* Add comprehensive field classification to dataprotection ([30bcd29](https://github.com/pay-theory/lift/commit/30bcd299ed800bd15ada6f9772b04fc8a82bcc3f))
* Add comprehensive idempotency middleware documentation and examples ([c0befe7](https://github.com/pay-theory/lift/commit/c0befe78561fff7712b5d386bb7d8ede37b2b32f))
* Add comprehensive README to basic-api template, streamline Go module handling in CI/CD and CDK, and remove `constructs-go/constructs/v10` dependency. ([17cb015](https://github.com/pay-theory/lift/commit/17cb0154019ae693f6b10cf7c76fe6a06d6e1b47))
* add configurable CORS origins to API construct ([bfac375](https://github.com/pay-theory/lift/commit/bfac375c8ead103b1c19c0a64b7b8e4b65fb11f8))
* add configurable VPC endpoints to enhanced security ([d95665b](https://github.com/pay-theory/lift/commit/d95665bd945c1832b5391d9ef0467c11ff5a0eae))
* add domain management constructs for API Gateway ([02db1a3](https://github.com/pay-theory/lift/commit/02db1a310089d3b975a954e5f1b8b4c094524261))
* add event source mapping construct with cross-region support ([ada7432](https://github.com/pay-theory/lift/commit/ada7432291d80cb8236d2521375c017e1ac0e8a4))
* Add extensive test coverage across features, constructs, and CLI, and introduce a new DynamoDB facade. ([221d5b5](https://github.com/pay-theory/lift/commit/221d5b578f3b26bdd55d13e555e2b69981df3863))
* Add IP authorization security feature with AWS SSM integration ([22ce388](https://github.com/pay-theory/lift/commit/22ce388c9da58b9d00642cc9a70bc186eba7c76a))
* Add IP authorization security feature with AWS SSM integration ([5987ab4](https://github.com/pay-theory/lift/commit/5987ab49c0026ff7b028ff4d51a751622c68fbd4))
* Add IsLambda() helper method to App structure ([e7bcaac](https://github.com/pay-theory/lift/commit/e7bcaacfe48a5e750c6372667fb9236383e97db7))
* add Lambda CloudWatch alarms construct ([1a0e942](https://github.com/pay-theory/lift/commit/1a0e94270df754776d5738c5e7a675dc832637ec))
* add Lambda CloudWatch alarms construct ([ec71358](https://github.com/pay-theory/lift/commit/ec713580114d6b8095f341be5e835dba956a6ae8))
* add Lambda IAM role construct with comprehensive permissions ([21b73a9](https://github.com/pay-theory/lift/commit/21b73a93890e925c2dc043a9fdb9238148e7e821))
* Add LiftRestAPI construct for API Gateway v1 and implement response streaming (SSE) support with accompanying documentation. ([cd69f6b](https://github.com/pay-theory/lift/commit/cd69f6bce1f466acc5e1d75fc519dbf112508ade))
* Add LoggerConfig helper functions with configurable log level ([7a15a1d](https://github.com/pay-theory/lift/commit/7a15a1d9eebd72d8f88eca47a2531699093b9606))
* add multi-region KMS key construct with HMAC support ([d0e3849](https://github.com/pay-theory/lift/commit/d0e38493255e495665dcf33350d67bbe1a98e15e))
* Add mutex protection to EventRouter for thread safety ([57532e0](https://github.com/pay-theory/lift/commit/57532e00ffa8814f0fa1785f79665d2cb1b3fb70))
* Add partner and target mode support to `up` and `down` commands, introduce new `basic-api-pt` project templates, and enforce partner flag usage. ([47bc7b0](https://github.com/pay-theory/lift/commit/47bc7b08256bc058fde24bfcd307fbc21b3d8927))
* Add per-method streaming overrides to `LiftRestAPI` integration… ([3cbeca4](https://github.com/pay-theory/lift/commit/3cbeca42a28d15df453481094b584ce94bb0cdf0))
* Add per-method streaming overrides to `LiftRestAPI` integrations and update `SSEResponse` for API Gateway v1 streaming. ([796689f](https://github.com/pay-theory/lift/commit/796689fb8e9101974d52ea33000ad786f9274a92))
* Add prerequisite checks to CLI commands, new getting started and CI documentation, and a release workflow. ([4dab741](https://github.com/pay-theory/lift/commit/4dab7411d730618aed440951e0800c6afb32d398))
* Add response interception support for middleware ([a0a3f14](https://github.com/pay-theory/lift/commit/a0a3f14f7ca817b6ebc7f76cfc4ca764bebf88f8))
* Add response interception support for middleware ([c2a7c35](https://github.com/pay-theory/lift/commit/c2a7c35b93a6d045a17fba35741af54653b09def))
* add REST API support and refactor common API utilities ([13c4972](https://github.com/pay-theory/lift/commit/13c4972f671b8f006a3bdb7bbb6b075c9430f657))
* add REST API support and refactor common API utilities ([7aecad1](https://github.com/pay-theory/lift/commit/7aecad126dd1cf400575d997e30a5cbd04fdbf52))
* Add RunLocalTest method for local event testing ([007a884](https://github.com/pay-theory/lift/commit/007a88465bb8e4895f3da3f049cb66a248c254e5))
* Add RunLocalTest method for local event testing ([7e88942](https://github.com/pay-theory/lift/commit/7e8894219360dcd39c601053ccb645a95e71d760))
* Add S3 object key pattern matching support ([774b328](https://github.com/pay-theory/lift/commit/774b3286b0948617521d2b3290c8fafb0dd5ca2e))
* add S3 object store for large SQS payloads and make SHA256 verification option nullable in envelope. ([7849962](https://github.com/pay-theory/lift/commit/7849962242bcc607ebb596cb0e417b862afe606a))
* Add SNS error notification support to CloudWatch logger ([a372d77](https://github.com/pay-theory/lift/commit/a372d777f32a8ab43b440b08ae8f3bb7bb0d576c))
* Add SNS processor template and update CDK best practices documentation ([f0bff79](https://github.com/pay-theory/lift/commit/f0bff79e4911cde2a26cdf04e192c4def04df12f))
* add SQS large payload support via S3 offloading and integrate into SQSProcessor construct. ([d2c3215](https://github.com/pay-theory/lift/commit/d2c321505518b757ccd28bbc1b3eb86503084f2f))
* Add SQS large payload support with S3 bucket, lifecycle, and IAM permissions. ([57f090e](https://github.com/pay-theory/lift/commit/57f090eb2da3e7f87af9866ce83b0d21d029c7ed))
* add SQS queue construct with comprehensive configuration ([3d31e29](https://github.com/pay-theory/lift/commit/3d31e2955d88f4ce3614c3bb604983f4410add23))
* Add support for AWS SQS Extended Client large payload parsing and processing. ([1fb00fe](https://github.com/pay-theory/lift/commit/1fb00fe5603d34f1ca0a699d695fa69ba07e3f2a))
* add WebSocket action routing, introduce naming utilities, and enhance EventBus with DynamoDB durability. ([79d3e22](https://github.com/pay-theory/lift/commit/79d3e2291e6e92eb0743016149d9717ee190f492))
* Add working rate limiting middleware using limited library ([d45a43d](https://github.com/pay-theory/lift/commit/d45a43de2228848f3a70bf5051c022ae94041ad5))
* Add working rate limiting middleware using limited library ([b290c07](https://github.com/pay-theory/lift/commit/b290c071eb426ab1d540be774bf3750af0fee013))
* comprehensive security fixes and framework improvements ([813d693](https://github.com/pay-theory/lift/commit/813d693a3013ae06e6740c0a94e1757df6a12d99))
* Enhance API Gateway REST API (v1) streaming with `multiValueHeaders` support, per-method timeout overrides, and documentation for throughput limits and heartbeats. ([86f7f84](https://github.com/pay-theory/lift/commit/86f7f845903069b92cec072585f23af73f99510e))
* enhance DynamoDB construct with GSI, encryption, and auto-scaling ([0eb9e23](https://github.com/pay-theory/lift/commit/0eb9e2342dfb83a686899c7a8fedc2ccbed60802))
* Enhance LiftError Error() method with cause and details ([b7b051b](https://github.com/pay-theory/lift/commit/b7b051bc2ab45b8b6e919772af68b635f7c7ce0b))
* implement event routing fix for non-HTTP Lambda events ([6f81d40](https://github.com/pay-theory/lift/commit/6f81d409b9339fc0d13212ff7edc72ea531b8e11))
* Implement SQS batch processor to manage large payloads stored in S3, supporting hydration, message processing, and object deletion. ([3de39db](https://github.com/pay-theory/lift/commit/3de39db4ec65450391aa06c3f72f71fd92136a01))
* Improve sanitization with blocklist approach and nested JSON ha… ([f59df09](https://github.com/pay-theory/lift/commit/f59df090a2986ca68c7de8306d31142e9530f880))
* Improve sanitization with blocklist approach and nested JSON handling ([0257a6c](https://github.com/pay-theory/lift/commit/0257a6c1e8be102bd41548d2a30d76e5cec07f51))
* introduce `add` command to CLI and add dynamic function placeholders to project templates. ([edf73be](https://github.com/pay-theory/lift/commit/edf73beba2a017d7cbe9b5ad9be1ec919d15efc1))
* Introduce `down` command for infrastructure teardown, alongside new state management and domain resolution capabilities. ([c3a9736](https://github.com/pay-theory/lift/commit/c3a97367c163498395431729a2623268703aafc9))
* Introduce CloudFront helper functions and enhance event bus components. ([4ff9c21](https://github.com/pay-theory/lift/commit/4ff9c2153fd6304af372d97ff9369b83bfb197a3))
* Introduce comprehensive event bus streaming, scheduling, and processing capabilities with new CDK constructs and documentation, alongside updates to idempotency middleware and CDN patterns. ([d24b396](https://github.com/pay-theory/lift/commit/d24b396914e9032819a15fefcb4ad387feb48e8c))
* Introduce DynamoDB stream event handling with a dedicated registration method and context record parsing. ([1d29d61](https://github.com/pay-theory/lift/commit/1d29d6113c55d06c38b05fc3ddc431f4517e82d4))
* introduce lift CLI with new and up commands to bootstrap projects and manage deployments. ([8a622dd](https://github.com/pay-theory/lift/commit/8a622dd03dce9be4478dd2731284729b297bd2d0))
* Introduce new project templates for event-driven, microservice, and merchant applications, and update CLI commands and embedding for template management. ([a4fcc84](https://github.com/pay-theory/lift/commit/a4fcc8419e8863dfeaf7b9e3c95a60e1eb426ed1))
* introduce SQS large payload support via S3 offloading with new documentation and an example. ([adaad7f](https://github.com/pay-theory/lift/commit/adaad7f923468b7d52e842b09c6a6a2b53ed4979))
* optimize struct field alignment to reduce memory usage ([f117034](https://github.com/pay-theory/lift/commit/f117034e63aa98cece9da39d4fe54fa2ee728aef))
* prevent IP address redaction in data protection ([9ba8d97](https://github.com/pay-theory/lift/commit/9ba8d97d63d45b20bf4e47239be19d737f0176f3))
* prevent key fields from being redacted in logs ([a92cd6e](https://github.com/pay-theory/lift/commit/a92cd6ee6640877f064e91a72041b8ade995cc31))
* prevent key fields from being redacted in logs ([d63e40e](https://github.com/pay-theory/lift/commit/d63e40edd848c3bc1658e4aa9e913d556bbd2d40))
* Return raw SQS batch responses and propagate SQS handler errors for SQS triggers. ([4e65ed7](https://github.com/pay-theory/lift/commit/4e65ed7410f9c4f215b92990a4b33f32f48d25c9))
* **sanitization:** add cardholder and cardholder_name to fully redacted PII fields ([0d34564](https://github.com/pay-theory/lift/commit/0d34564ba1d67947436e42bd5160a8f47e47607a))
* Show BIN (first 6 digits) + last 4 in card sanitization ([c42b9ca](https://github.com/pay-theory/lift/commit/c42b9ca724949b1269624240c1e4e3c98ac5eec0))
* universal SNS error notifications for all Pay Theory services ([7fee7fd](https://github.com/pay-theory/lift/commit/7fee7fd86aa6a988c2a86c0ccfdb8eb1f238bf88))
* universal SNS error notifications for all Pay Theory services ([c08cf43](https://github.com/pay-theory/lift/commit/c08cf4310dd22856e173f42f8cfebff65d66427b))


### Bug Fixes

* add austin partner to qakernel routing ([5d0a554](https://github.com/pay-theory/lift/commit/5d0a55462060b5da45da77ce95aa79b29d901b04))
* add AWS region detection to Limited rate limit helper functions ([4bc0e0a](https://github.com/pay-theory/lift/commit/4bc0e0ab00cff6aebacd12fafee7186fc0b377ee))
* add AWS region detection to Limited rate limit helper functions ([9c6dbed](https://github.com/pay-theory/lift/commit/9c6dbed5e090d9cec5a16c255e8592f1cd17ff21))
* add error handling for 108 critical errcheck violations ([235d5ac](https://github.com/pay-theory/lift/commit/235d5aceb6dd0ee6a67b73916341928a571cc149))
* add missing dist directory for CDK construct tests ([97c6148](https://github.com/pay-theory/lift/commit/97c6148b4fa35c55a0a98f05c238427ab716453c))
* add proper error handling for critical operations ([cc55fa2](https://github.com/pay-theory/lift/commit/cc55fa2cf82faebf82fa83bc9f7447d2609b7d02))
* align Lift CDK tables with DynamORM field name conventions ([3a3911b](https://github.com/pay-theory/lift/commit/3a3911be73b3a81b8b858fb922a1cbe599db54e0))
* apply content-based deduplication to FIFO dead-letter queues ([89dcfcb](https://github.com/pay-theory/lift/commit/89dcfcb789d3e77ab596df080f1674781ca0d862))
* apply content-based deduplication to FIFO dead-letter queues ([60c9306](https://github.com/pay-theory/lift/commit/60c9306e2f32ea78b271f5c8cca1066e5a866df2))
* **ci:** align release line to v1.1.0 ([453b13d](https://github.com/pay-theory/lift/commit/453b13d2b188625d1d3856e2a9cae78e5901f3f7))
* **ci:** publish release assets via github.token ([996a3bd](https://github.com/pay-theory/lift/commit/996a3bd1bb5d15bbeb510db608c62e7968de0071))
* **ci:** publish release assets via github.token ([084f079](https://github.com/pay-theory/lift/commit/084f0796bca751000618cdd1574a06dd15c9c7d5))
* **ci:** support immutable GitHub releases ([01c6855](https://github.com/pay-theory/lift/commit/01c68559f9b045816ef82d245b19319d3385dac8))
* **ci:** support immutable GitHub releases ([d2fb7ca](https://github.com/pay-theory/lift/commit/d2fb7ca9453f8acbdf2c60f5825e611f0fae5e25))
* **cli:** mock CDK/Go presence in tests to support CI environments without them ([fb66ad2](https://github.com/pay-theory/lift/commit/fb66ad28b20bb256a499a5ef2cf1be6eeb9c62a7))
* comprehensive error handling for all errcheck violations ([c38dca9](https://github.com/pay-theory/lift/commit/c38dca98cc3945da53f3481f50291a7b33d1ca27))
* Convert partner and stage to lowercase in SNS notifications ([1bea7fa](https://github.com/pay-theory/lift/commit/1bea7faceb399cffd9bb69538a8ddbbd541509e7))
* correct stage prefix stripping to avoid false matches ([d440086](https://github.com/pay-theory/lift/commit/d440086209d5a4a5e03598de3e47a8c1931fa922))
* Exclude card_bin from log redaction as it's not sensitive data ([ca71e64](https://github.com/pay-theory/lift/commit/ca71e642d77300f5fa4eab113ebc9ab1e307556d))
* Fix idempotency middleware test failures ([0c1c311](https://github.com/pay-theory/lift/commit/0c1c311e56f0115e59f341264b07f82ff375db26))
* Fix idempotency middleware test failures ([833eeb1](https://github.com/pay-theory/lift/commit/833eeb13ea4acf9d19d8c2a8f0c65abe321effed))
* Fix WebSocket routing in HandleRequest method ([f84a883](https://github.com/pay-theory/lift/commit/f84a8839c35526ca4e187c1cd3f21d10065168f0))
* Fix WebSocket routing in HandleRequest method ([18d4d25](https://github.com/pay-theory/lift/commit/18d4d25a029ecac823a5cb9b985b0649753d69af))
* Handle API Gateway v2 stage prefix in paths for custom domains ([c210756](https://github.com/pay-theory/lift/commit/c210756172eaf3831bfd616cb9834948b7273f44))
* improve API Gateway adapter detection and error handling ([de89abc](https://github.com/pay-theory/lift/commit/de89abcf2b6479692e1522ec7358c941bae0e3a4))
* improve API Gateway adapter detection and error handling ([d8c1885](https://github.com/pay-theory/lift/commit/d8c1885995a393fde51634f2aaa4edde2436d8d1))
* improve API Gateway stage management and domain mapping ([af18cae](https://github.com/pay-theory/lift/commit/af18caefb39cd2ee9a48da73dd74122746fb852d))
* improve API Gateway stage management and domain mapping ([28c0ac7](https://github.com/pay-theory/lift/commit/28c0ac764c2e962f00518328305e7bc42607c5d3))
* Improve sensitive data redaction in CloudWatch logger ([6961d1b](https://github.com/pay-theory/lift/commit/6961d1b700faf1ab80d17b4367e9a9542b2afca8))
* Initialize real logger in Logger middleware when ctx.Logger is nil ([5f2161d](https://github.com/pay-theory/lift/commit/5f2161dc2fd2aa2a24fe124bd35c61982813565e))
* Initialize real logger in Logger middleware when ctx.Logger is nil ([d222ed0](https://github.com/pay-theory/lift/commit/d222ed0e225be0ce7bcd8c68ac92d78599f5bbc3))
* install proper gosec version and fix golangci-lint config ([6033af5](https://github.com/pay-theory/lift/commit/6033af545246d5d995f9f902b1133443ac7f9af2))
* Middleware not executing in Lambda HandleRequest + Example compilation fixes ([51c408c](https://github.com/pay-theory/lift/commit/51c408c39ff939bc83890cdc8e9bf6869e763c93))
* minimize WebSocket CDK resource IDs to prevent CloudFormation errors ([1fbabf2](https://github.com/pay-theory/lift/commit/1fbabf2fdf56ed72394ff103483a623648fff1c3))
* minimize WebSocket CDK resource IDs to prevent CloudFormation name length errors ([7122fe5](https://github.com/pay-theory/lift/commit/7122fe5f65b0499a3e89db931b7e59f0a7cc5242))
* optimize gosec scope to prevent timeouts ([433715e](https://github.com/pay-theory/lift/commit/433715e9eddc081f0d81ed6f95e206c0feb3e28e))
* optimize struct field alignment to reduce memory usage ([e86ce95](https://github.com/pay-theory/lift/commit/e86ce95d58b6b851af70bc29872dcb536e3c9e0f))
* prevent ID fields from being redacted in logs ([114acb4](https://github.com/pay-theory/lift/commit/114acb4b2065a4f0cd6b547d0efb89b9ccaa9a8b))
* prevent ID fields from being redacted in logs ([a9d6473](https://github.com/pay-theory/lift/commit/a9d6473a2a0ba07ad8ceefe849837818d570e211))
* properly fix WebSocket CDK resource name length issue ([3ca78ee](https://github.com/pay-theory/lift/commit/3ca78ee81c6f026a7a3929fb217c516aaf431a88))
* redesign MultiTenantAPI to use DynamORM-compatible tables ([0db11f9](https://github.com/pay-theory/lift/commit/0db11f9f7e571ba7c18b0960ab390aa6393b34d8))
* remove all log-related parameters from Lambda constructs ([ce3c8de](https://github.com/pay-theory/lift/commit/ce3c8dee4e4eef6e5d93e36493a41252e5ea9f3a))
* remove all log-related parameters from Lambda constructs ([b3bbf63](https://github.com/pay-theory/lift/commit/b3bbf63944dfc0bfa4fddcc85ba975dfde9c19ed))
* remove all log-related parameters from Lambda constructs and fix mutex issues ([d41a1b8](https://github.com/pay-theory/lift/commit/d41a1b86b3fa16e7da965a84596973a69028efa7))
* remove broken MultiTenantAPI construct ([2c5ab4d](https://github.com/pay-theory/lift/commit/2c5ab4dc72023df5dc48631ce60aa25009ba676e))
* remove test file with non-existent dynamorm/v2 import ([e1c38fd](https://github.com/pay-theory/lift/commit/e1c38fdf07b1d449e6cdd723c9184571502bb668))
* replace deprecated gofmt linter with gofumpt ([ca230ee](https://github.com/pay-theory/lift/commit/ca230ee4daffc50d2ff81063a4ab47bfeebc1e18))
* resolve 22 staticcheck violations for correctness and performance ([4f825af](https://github.com/pay-theory/lift/commit/4f825afd94537081ed9c9d8b7b7b221bdfc4bf82))
* resolve 442 govet issues including critical shadow variables ([b85e2f5](https://github.com/pay-theory/lift/commit/b85e2f510fc146bf35d4b61726e2b3de7bcd087e))
* resolve 75 critical errcheck violations in production code ([a0b36cc](https://github.com/pay-theory/lift/commit/a0b36ccef39b9d050668e636d6e48619e6df5691))
* resolve 89 critical errcheck violations in production paths ([db47eeb](https://github.com/pay-theory/lift/commit/db47eeb0735fbe9a0c2c61ede9e595291962a8c6))
* resolve additional lint violations for code quality ([cb251fb](https://github.com/pay-theory/lift/commit/cb251fb0e52bffa1e25787dddfc30313e5b9ddc5))
* resolve all 42 gosec security violations ([b1fff0f](https://github.com/pay-theory/lift/commit/b1fff0fe2a7208f4fb9c2fc308202324b1a732d0))
* resolve all failing tests and complete DynamORM table standardization ([c4990cd](https://github.com/pay-theory/lift/commit/c4990cd649defbe26aad28fba0572d9b49b705a0))
* resolve all failing tests and complete DynamORM table standardization ([7553aa1](https://github.com/pay-theory/lift/commit/7553aa1b49504fd9986103519f690c4212a7585e))
* resolve compilation error in debug logging ([9955509](https://github.com/pay-theory/lift/commit/99555091e495e328ca7f19117d30d9c40d0aeceb))
* resolve critical govet correctness issues ([70be218](https://github.com/pay-theory/lift/commit/70be2181f23c8366cf87e6571dcdd1c0a6cbf5f9))
* resolve critical govet issues and optimize struct alignment ([5822ed2](https://github.com/pay-theory/lift/commit/5822ed28ca86d62b541fcbd28d86a43476620fa5))
* resolve critical govet shadow variable and fieldalignment violations ([d9cfa9f](https://github.com/pay-theory/lift/commit/d9cfa9fa005a6b04fa379c598f793bb01374d86c))
* resolve critical govet shadow variable and optimize field alignment ([87b5ca7](https://github.com/pay-theory/lift/commit/87b5ca7b54619e8edb620d05bf06fb70158f33ce))
* resolve critical govet shadow variable violations ([96830e3](https://github.com/pay-theory/lift/commit/96830e3dbb43e53527fdae9d723bbe172bef8fb2))
* resolve critical lint violations for production reliability ([bf1b9b8](https://github.com/pay-theory/lift/commit/bf1b9b8fde0a41fc38edeb0836ac00ad58b301a6))
* Resolve duplicate CloudWatch logs during logger shutdown ([28ff5e8](https://github.com/pay-theory/lift/commit/28ff5e8a831248426b7a3cbc9e4bb5eb975b2c68))
* Resolve LiftError status codes not being properly mapped to HTTP responses ([fa0faac](https://github.com/pay-theory/lift/commit/fa0faaca7996c75fb9ac12a2a218bb949740f3a1))
* Resolve LiftError status codes not being properly mapped to HTTP responses ([79279d2](https://github.com/pay-theory/lift/commit/79279d26af03f6a9aa78ba4406b084a275e0ff5a))
* resolve LogGroup duplication issue in LiftFunction construct ([0764965](https://github.com/pay-theory/lift/commit/076496550d53427b79a4f0a1a057ff053c236b0b))
* resolve LogGroup duplication issue in LiftFunction construct ([df5da1a](https://github.com/pay-theory/lift/commit/df5da1a2ec5d98556d9690928b228799d029ff2b))
* resolve remaining critical govet issues ([6e3c993](https://github.com/pay-theory/lift/commit/6e3c993e9e249b71caf8930b650add6b51eade0c))
* resolve remaining lint violations and reduce from 3000+ to 120 issues ([029d10c](https://github.com/pay-theory/lift/commit/029d10c0979b835a501d9d807fbb281e21e1c006))
* resolve test failures across multiple packages ([7df2b63](https://github.com/pay-theory/lift/commit/7df2b63f7df76c20746d9d43559e9753371262fc))
* resolve test failures across multiple packages ([edd1127](https://github.com/pay-theory/lift/commit/edd1127d25d1d572fe27aa68742f4224cd5df653))
* update CDK construct tests to use DynamORM field naming standards ([f878e85](https://github.com/pay-theory/lift/commit/f878e85be1af7126e70eb4e6eedabfd11d451388))
* update CDK construct tests to use DynamORM field naming standards ([ac1ee45](https://github.com/pay-theory/lift/commit/ac1ee458486639ec6d234b9f3bde88d7679d9a01))
* update test to expect standard pk naming for multi-tenant tables ([802a798](https://github.com/pay-theory/lift/commit/802a7988a3f6eecffd1075a6268af12e215d5f08))
* use RemovalPolicy_DESTROY instead of non-existent RemovalPolicy_DELETE ([f56175a](https://github.com/pay-theory/lift/commit/f56175a2981b3b1dfca85206cf8cd68762ad6b01))
* use standard pk/sk naming for DynamoDB multi-tenant tables ([802df1f](https://github.com/pay-theory/lift/commit/802df1f7c11e6e597faa283fd4bbaed1b0dd7e2f))


### Code Refactoring

* make VPC authorizer generic by accepting ARNs directly ([fcc7968](https://github.com/pay-theory/lift/commit/fcc7968a8563e1bf6ff68cfd36db5e5313556da7))

## [1.1.0](https://github.com/pay-theory/lift/compare/v1.0.84...v1.1.0) (2026-01-14)


### Bug Fixes

* **ci:** align release line to v1.1.0 ([453b13d](https://github.com/pay-theory/lift/commit/453b13d2b188625d1d3856e2a9cae78e5901f3f7))
* **ci:** publish release assets via github.token ([996a3bd](https://github.com/pay-theory/lift/commit/996a3bd1bb5d15bbeb510db608c62e7968de0071))
* **ci:** publish release assets via github.token ([084f079](https://github.com/pay-theory/lift/commit/084f0796bca751000618cdd1574a06dd15c9c7d5))
* **ci:** support immutable GitHub releases ([01c6855](https://github.com/pay-theory/lift/commit/01c68559f9b045816ef82d245b19319d3385dac8))
* **ci:** support immutable GitHub releases ([d2fb7ca](https://github.com/pay-theory/lift/commit/d2fb7ca9453f8acbdf2c60f5825e611f0fae5e25))

## [1.0.85-rc](https://github.com/pay-theory/lift/compare/v1.0.84...v1.0.85-rc) (2026-01-14)


### Bug Fixes

* **ci:** publish release assets via github.token ([996a3bd](https://github.com/pay-theory/lift/commit/996a3bd1bb5d15bbeb510db608c62e7968de0071))
* **ci:** publish release assets via github.token ([084f079](https://github.com/pay-theory/lift/commit/084f0796bca751000618cdd1574a06dd15c9c7d5))

## v1.0.82 - 2026-01-04

### Added

- **Lift CLI**: Full implementation of the `lift` command-line tool for project scaffolding and deployment:
  - `lift new <project-name>` - Bootstrap new Lift projects with pre-configured templates
  - `lift build` - Compile Lambda functions with configurable build settings in `lift.yaml`
  - `lift up` - Build and deploy CDK stacks with domain immutability enforcement
  - `lift down` - Destroy stacks in reverse order and clear state files
  - `lift add function` - Add new Lambda functions to existing projects with dynamic placeholders
  - Prerequisite checks for Go and CDK availability with actionable error messages
  - Partner mode (`--pt`) and target mode (`--partner`, `--target-mode`) support for Pay Theory deployments
- **Project Templates**: New project templates for common use cases:
  - `basic-api` - Standard REST API template with GitHub Actions CI/CD
  - `basic-api-pt` - Pay Theory-specific variant with CodeBuild pipelines
  - `event-driven` - Event-driven architecture template
  - `microservice` - Microservice template with enhanced configuration
  - `merchant-app` - Merchant application template
- **CDK Constructs**:
  - `PathRoutedFrontendDistribution` - CloudFront distribution with path-based routing
  - DynamoDB stream event handling with dedicated registration and context record parsing
  - CloudFront helper functions and enhanced event bus components
  - SNS processor template for error notifications
- **Event Bus**: Comprehensive event bus streaming, scheduling, and processing capabilities with new CDK constructs, DynamoDB durability, and idempotency middleware updates
- **WebSocket**: Action routing support with naming utilities
- **DynamoDB Facade**: New facade for simplified DynamoDB interactions
- **Testing Utilities**: Extensive middleware and enterprise testing utilities, security and circuit breaker test coverage improvements
- **Documentation**: New comprehensive guides for CDK, CLI, IP gating, error notifications, JWT patterns, and getting started

### Changed

- **CloudWatch**: Modularized dashboard row creation for Lambda monitoring with new helper functions
- **Go Modules**: Streamlined Go module handling in CI/CD and CDK workflows; removed `constructs-go/constructs/v10` dependency
- **Documentation**: Complete restructure for clarity; removed outdated development notes
- **CI/CD**: Added release workflow and updated GitHub Actions configurations

### Fixed

- **CLI Tests**: Mock CDK/Go presence in tests to support CI environments without local installations

## v1.0.81 - 2025-12-21

### Added
- **CDK (LiftRestAPI)**: Per-method response streaming overrides via `IntegrationOptions.EnableStreaming` and `IntegrationOptions.StreamingTimeoutSeconds`, enabling mixed buffered + streaming methods on the same REST API.

### Changed
- **Runtime (SSE)**: `lift.SSEResponse` returns `events.APIGatewayProxyStreamingResponse` for API Gateway REST API (v1) triggers (keeps `events.LambdaFunctionURLStreamingResponse` for other trigger types).
- **Runtime (SSE)**: Added `ctx.AddMultiValueHeader(...)` support for API Gateway REST API (v1) streaming responses (`multiValueHeaders` metadata).
- **Dependencies**: Updated `github.com/aws/aws-lambda-go` to include `events.APIGatewayProxyStreamingResponse`.
- **Docs**: Clarified per-method streaming configuration (`ResponseTransferMode=STREAM` + `/response-streaming-invocations`) and documented key platform limits (15m integration timeout, idle timeouts).

## v1.0.80 - 2025-12-21

### Added
- **Response Streaming**: Added Server-Sent Events (SSE) support for Lambda response streaming via `LiftRestAPI` construct. See `docs/response-streaming.md` for usage.
- **LiftRestAPI Construct**: New CDK construct for API Gateway v1 with enhanced configuration options and streaming support.
- **IP Gating Documentation**: Added comprehensive documentation for IP authorization and error handling patterns.

### Changed
- **Observability** (BREAKING): Removed `WithDefaultErrorNotifications` function which contained hardcoded internal account IDs. Replaced with `WithEnvironmentErrorNotifications` which reads the SNS topic ARN from the `ERROR_NOTIFICATION_SNS_TOPIC_ARN` environment variable. See migration guide at `/docs/migration-sns-error-notifications.md`.
- **REST API Configuration**: Refactored REST API configuration into dedicated helper methods for improved maintainability.
- **Streaming Error Handling**: Improved error handling for streaming responses.

## v1.0.71 - 2025-10-27

### Added
- **Event Bus**: Introduced DynamoDB-backed event bus service with CDK constructs, helper utilities, and documentation. Includes DynamoDB integration helpers and comprehensive service tests.
- **Streamer Client**: Added streamer client library with connection lifecycle management, structured errors, mocks, and demo application.
- **Documentation**: Added event bus and streamer guides along with new LLM FAQ entries and planning notes.

### Changed
- **WebSocket Context**: Migrated management client to AWS SDK v2 with connection metadata helpers, thread-safe reuse, and improved region resolution.
- **Testing & Tooling**: Expanded mocks, added load-shedding and observability test coverage, and refreshed Go module dependencies.

## v1.0.70 - 2025-01-15

### Added
- **Kernel Client**: Added `pkg/services/kernel` package for authenticated cross-account calls to kernel services. Provides SigV4-signed API Gateway calls with STS AssumeRole authentication, matching Python's `secure_api_call.py` pattern. Includes convenience functions for K3, Paze Wallet, Apple Wallet, Google Wallet, Bin Lookup, and Bank Data services. Supports both shared role (`kernel-access`) and external partner role (`kernel-access-external`) authentication modes. Uses singleton logger pattern via LoggerFunc. See `pkg/services/kernel/doc.go` and `examples/kernel_client_example.go` for usage.

### Changed
- **Observability**: Updated SNS notification APIs. See v1.0.72+ for the current API.
- **Observability**: Added `WithPartnerErrorNotifications` for services that need partner-specific SNS topics (`cns-{partner}-{stage}`). This includes AWS account ID auto-detection via STS GetCallerIdentity when `AWS_ACCOUNT_ID` environment variable is not set. Most services should use `WithDefaultErrorNotifications` instead.
- Router: Unmatched HTTP routes now return structured 404 `LiftError` instead of a generic error.
- Response: `Binary` responses are correctly base64-encoded and flagged with `isBase64Encoded=true`; JSON marshalling respects base64 mode.
- Middleware (Lift IP Authorization): Stop writing responses via deprecated context helpers; now returns `LiftError`s (`ParameterError`, `SystemError`, `AuthorizationError`).
- Health Endpoints: Added optional structured logging support via `HealthEndpointsConfig.Logger`; falls back to standard logging when not provided.
- Docs sweep for accuracy and consistency:
  - Replace `middleware.JWT(...)` with `middleware.JWTAuth(...)` and remove erroneous error returns.
  - Replace `ctx.Bind(...)` with `ctx.ParseRequest(...)`.
  - Use `lift.NewLiftError(...)` and dedicated error constructors (`ValidationError`, `NotFound`, `SystemError`) instead of `NewError`/`BadRequest` patterns.
  - Fix response examples to use `ctx.JSON(data)` for 200 or `ctx.Status(code).JSON(data)` otherwise.
  - Update training and guidance files: `_patterns.yaml`, `_decisions.yaml`, `troubleshooting.md`, `migration-guide.md`, `api-reference.md`, `core-patterns.md`, `dynamorm-integration.md`, `README.md`, `development-guidelines.md`, `cdk/event-driven-api-pattern.md`.
- Tests: Added tests for router 404 behavior and binary response encoding.

### JWT Consolidation

- Canonical API is `middleware.JWTAuth(middleware.JWTConfig)`.
- `lift.WithJWTAuth` and `lift.WithSimpleJWTAuth` are now deprecated; see `docs/migration-jwt-consolidation.md`.

> Note: `HealthEndpointsConfig` gained an optional `Logger` field; this is backwards-compatible. Existing uses continue to work without changes.
