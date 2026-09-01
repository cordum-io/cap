package releasetruth

import "strings"

// canonicalRegistryProof locks the fail-closed shell program after comments are
// removed. A looser token check can preserve every expected word while adding
// an early success, function override, or failure-counter reset.
const canonicalRegistryProof = `      - name: Registry exact-version proof
        shell: bash
        run: |
          set -uo pipefail
          VERSION="$(python -c 'import json; print(json.load(open("release/manifest.json", encoding="utf-8"))["release"]["version"])')"
          if ! [[ "$VERSION" =~ ^2\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
            echo "manifest release version is not stable v2: ${VERSION}" >&2
            exit 1
          fi
          echo "manifest release version: ${VERSION}"
          check() {
            local name="$1" url="$2" code="" curl_status=0
            for attempt in 1 2 3 4 5; do
              curl_status=0
              code="$(curl --silent --show-error --output /dev/null --write-out '%{http_code}' --max-time 15 "$url")" || curl_status=$?
              case "$code" in
                200) echo "${name}: FOUND"; return 0 ;;
                404|410) echo "${name}: ABSENT (${code})" >&2; return 1 ;;
                *) echo "${name}: UNKNOWN (curl=${curl_status}, http=${code:-none}), retry ${attempt}/5" >&2 ;;
              esac
              sleep 5
            done
            echo "${name}: UNKNOWN after bounded retries" >&2
            return 1
          }
          failures=0
          check "go v${VERSION}" "https://proxy.golang.org/github.com/cordum-io/cap/v2/@v/v${VERSION}.info" || failures=$((failures + 1))
          check "npm cap-sdk-node" "https://registry.npmjs.org/cap-sdk-node/${VERSION}" || failures=$((failures + 1))
          check "pypi cap-sdk-python" "https://pypi.org/pypi/cap-sdk-python/${VERSION}/json" || failures=$((failures + 1))
          check "pypi cordum-guard" "https://pypi.org/pypi/cordum-guard/${VERSION}/json" || failures=$((failures + 1))
          test "$failures" -eq 0
`

const canonicalBuildJob = `    name: Go installed consumer (${{ matrix.os }})
    runs-on: ${{ matrix.os }}
    timeout-minutes: 15
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-24.04, windows-2025]
    steps:
      - uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683
        with:
          persist-credentials: false
      - uses: actions/setup-go@d35c59abb061a4a6fb18e82ac0862c26744d6ab5
        with:
          go-version-file: 'go.mod'
          cache: false
      - uses: actions/setup-python@a26af69be951a213d495a4c3e4e4022e16d87065
        with:
          python-version: '3.11.9'
      - name: Verify Go consumer helper contract
        run: python -m unittest tools.registry_verify.test_go_consumer tools.registry_verify.test_process_runner
      - name: Build manifest-selected Go consumer
        run: python tools/registry_verify/go_consumer.py --manifest release/manifest.json --example examples/quickstart-echo/main.go --build-only`

const canonicalLiveJob = `    name: Registry and real-NATS installed quickstarts
    needs: build-proof
    if: ${{ needs.build-proof.result == 'success' }}
    runs-on: ubuntu-24.04
    timeout-minutes: 15
    services:
      nats:
        image: nats:2.12.6-alpine@sha256:1cfc36e2e5e638243d8c722f72c954cd0ec4b15ee82fadbc718ce12e2b3c1652
        ports:
          - 4222:4222
    env:
      CAP_NATS_URL: nats://127.0.0.1:4222
    steps:
      - uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683
        with:
          persist-credentials: false
      - uses: actions/setup-go@d35c59abb061a4a6fb18e82ac0862c26744d6ab5
        with:
          go-version-file: 'go.mod'
          cache: false
      - uses: actions/setup-python@a26af69be951a213d495a4c3e4e4022e16d87065
        with:
          python-version: '3.11.9'

` + canonicalRegistryProof + `
      - name: Go released-artifact quickstart
        run: python tools/registry_verify/go_consumer.py --manifest release/manifest.json --example examples/quickstart-echo/main.go --run

      - name: Python released-artifact quickstart
        shell: bash
        run: |
          set -euo pipefail
          VERSION="$(python -c 'import json; print(json.load(open("release/manifest.json", encoding="utf-8"))["release"]["version"])')"
          python -m venv "$RUNNER_TEMP/cap-registry-python"
          "$RUNNER_TEMP/cap-registry-python/bin/python" -m pip install --disable-pip-version-check "cap-sdk-python==${VERSION}" "nats-py==2.6.0"
          "$RUNNER_TEMP/cap-registry-python/bin/python" examples/quickstart-echo/echo.py`

func validateCanonicalRegistryJobs(violations *[]string, build, live string) {
	if strings.Trim(build, "\n") != canonicalBuildJob {
		*violations = append(*violations, "canonical build job")
	}
	if strings.Trim(live, "\n") != canonicalLiveJob {
		*violations = append(*violations, "canonical live job")
	}
}

type workflowMutation struct {
	name, old, replacement, want string
}

var registryWorkflowMutations = []workflowMutation{
	{"drop schedule", "  schedule:\n", "  disabled_schedule:\n", "schedule trigger"},
	{"drop dispatch", "  workflow_dispatch:\n", "  disabled_dispatch:\n", "workflow_dispatch trigger"},
	{"skip build job", "  build-proof:\n", "  build-proof:\n    if: ${{ 1 == 2 }}\n", "build proof job condition"},
	{"ignore matrix", "runs-on: ${{ matrix.os }}", "runs-on: ubuntu-24.04", "canonical build job"},
	{"drop Windows", "os: [ubuntu-24.04, windows-2025]", "os: [ubuntu-24.04]", "Windows/Linux build matrix"},
	{"exclude Windows", "        os: [ubuntu-24.04, windows-2025]", "        os: [ubuntu-24.04, windows-2025]\n        exclude:\n          - os: windows-2025", "canonical build job"},
	{"drop dependency", "    needs: build-proof", "    needs: unrelated", "live job depends on build proof"},
	{"skip live job", "needs.build-proof.result == 'success' }}", "needs.build-proof.result == 'success' && 1 == 2 }}", "successful matrix dependency"},
	{"disable services", "    services:", "    disabled_services:", "live NATS service"},
	{"wrong NATS port", "          - 4222:4222", "          - 4999:4222", "live NATS port"},
	{"wrong NATS URL", "CAP_NATS_URL: nats://127.0.0.1:4222", "CAP_NATS_URL: nats://127.0.0.1:4999", "live NATS URL"},
	{"mutable checkout", checkoutPin, "actions/checkout@v4", "immutable checkout pin"},
	{"mutable setup Go", setupGoPin, "actions/setup-go@v5", "immutable setup-go pin"},
	{"mutable setup Python", setupPyPin, "actions/setup-python@v5", "immutable setup-python pin"},
	{"drop checkout fence", "persist-credentials: false", "persist-credentials: true", "checkout credential isolation"},
	{"wrong Go version", "go-version-file: 'go.mod'", "go-version: 'stable'", "root Go version file"},
	{"float Python", "python-version: '3.11.9'", "python-version: '3.11'", "exact Python version"},
	{"float broker", natsPin, "nats:2.10", "immutable NATS 2.12.6 image"},
	{"drop unit gate", "python -m unittest tools.registry_verify.test_go_consumer", "echo skipped-unit", "helper unit gate"},
	{"skip unit gate", "      - name: Verify Go consumer helper contract\n", "      - name: Verify Go consumer helper contract\n        if: ${{ 1 == 2 }}\n", "helper unit gate"},
	{"drop build gate", " --build-only", " --run", "matrix build-only gate"},
	{"ignore build failure", " --build-only", " --build-only || true", "matrix build-only gate"},
	{"drop live gate", " --run", " --build-only", "live readonly run gate"},
	{"ignore live failure", " --run", " --run || true", "live readonly run gate"},
	{"skip live Go", "      - name: Go released-artifact quickstart\n", "      - name: Go released-artifact quickstart\n        if: ${{ 1 == 2 }}\n", "canonical live job"},
	{"skip live Python", "      - name: Python released-artifact quickstart\n", "      - name: Python released-artifact quickstart\n        if: ${{ 1 == 2 }}\n", "canonical live job"},
	{"ignore Python failure", "examples/quickstart-echo/echo.py", "examples/quickstart-echo/echo.py || true", "canonical live job"},
	{"override Python version", "          python -m venv", "          VERSION=2.15.0\n          python -m venv", "canonical live job"},
	{"allow unknown", `test "$failures" -eq 0`, "true", "ABSENT and UNKNOWN fail closed"},
	{"reset failures", `test "$failures" -eq 0`, "failures=0\n          test \"$failures\" -eq 0", "ABSENT and UNKNOWN fail closed"},
	{"allow absent", "404|410) echo", "404|410) return 0; echo", "ABSENT and UNKNOWN fail closed"},
	{"allow exhausted unknown", "UNKNOWN after bounded retries\" >&2\n            return 1", "UNKNOWN after bounded retries\" >&2\n            return 0", "ABSENT and UNKNOWN fail closed"},
	{"override check", "          failures=0", "          check(){ return 0; }\n          failures=0", "canonical registry proof"},
	{"arithmetic reset", `          test "$failures" -eq 0`, "          failures=$((0))\n          test \"$failures\" -eq 0", "canonical registry proof"},
	{"early success", `          test "$failures" -eq 0`, "          exit 0\n          test \"$failures\" -eq 0", "canonical registry proof"},
	{"pip upgrade", "Python released-artifact quickstart", "Python released-artifact quickstart\n        run: pip install --upgrade pip", "forbidden bypass"},
}
