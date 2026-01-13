#!/bin/bash
SRC="hgm-infra/evidence/types_full.go"
HEADER="package enterprise\n\nimport (\n\t\"context\"\n\t\"fmt\"\n\t\"time\"\n)\n"

# Common
echo -e "$HEADER" > pkg/testing/enterprise/types_common.go
# Skip original header/imports (lines 1-8)
sed -n '9,97p;743,760p' "$SRC" >> pkg/testing/enterprise/types_common.go

# Compliance
echo -e "$HEADER" > pkg/testing/enterprise/types_compliance.go
sed -n '98,410p;761,774p;903,916p' "$SRC" >> pkg/testing/enterprise/types_compliance.go

# Contract
echo -e "$HEADER" > pkg/testing/enterprise/types_contract.go
sed -n '411,520p;825,849p;917,1278p' "$SRC" >> pkg/testing/enterprise/types_contract.go

# Alerting
echo -e "$HEADER" > pkg/testing/enterprise/types_alerting.go
sed -n '521,669p;879,902p' "$SRC" >> pkg/testing/enterprise/types_alerting.go

# Chaos
echo -e "$HEADER" > pkg/testing/enterprise/types_chaos.go
sed -n '670,742p;775,824p;851,878p;1279,1825p' "$SRC" >> pkg/testing/enterprise/types_chaos.go