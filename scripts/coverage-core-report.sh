#!/usr/bin/env bash
set -euo pipefail

profile_in="${1:-coverage.out}"
profile_core="${2:-coverage.core.out}"

if [[ ! -f "${profile_in}" ]]; then
  echo "coverage profile not found: ${profile_in}" >&2
  exit 1
fi

# Filter out pkg/testing from coverage reporting while still allowing those tests
# to run normally as part of the suite.
awk 'NR==1{print;next} !/^github.com\/pay-theory\/lift\/pkg\/testing\//' "${profile_in}" > "${profile_core}"

echo "Core coverage (excluding github.com/pay-theory/lift/pkg/testing/**):"
go tool cover -func="${profile_core}" | tail -n 1

awk 'NR==1{next}{total+=$2; if($3==0) uncovered+=$2} END{printf "core_statements=%d core_uncovered=%d core_coverage=%.2f%%\n", total, uncovered, 100*(total-uncovered)/total}' "${profile_core}"

echo
echo "Packages below 90% (excluding pkg/testing):"
awk 'NR==1{next}{
  split($1,a,":"); f=a[1];
  dir=f; sub(/\/[^\/]+$/, "", dir);
  total[dir]+=$2;
  if($3==0) uncovered[dir]+=$2
}
END{
  for(dir in total){
    t=total[dir]; u=uncovered[dir];
    cov=100*(t-u)/t;
    if(cov<90){
      target=int(t*0.1);
      need=u-target;
      if(need<0) need=0;
      printf "%.1f%%\t%d/%d\tneed %d\t%s\n", cov, u, t, need, dir
    }
  }
}' "${profile_core}" | sort -n

