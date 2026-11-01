// End-to-end tests: Go starts the real server on a
// socket with the fixtures loaded, then runs Playwright once per
// *.test.e2e.ts file as a subtest, reloading fixtures in between.
// The harness also mounts POST /_test/reset-fixtures (only here, never in the
// app router), which tests/e2e/utils_e2e.ts calls before every test.
//
//	make test-e2e                       # all files
//	make test-e2e#access                # tests/e2e/access.test.e2e.ts
//	PLAYWRIGHT_PROJECT=firefox make test-e2e
package e2e

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"git.cu3rd.ru/cu3rd/db-access/models/unittest"
	"git.cu3rd.ru/cu3rd/db-access/routers"
	access_service "git.cu3rd.ru/cu3rd/db-access/services/access"
	"git.cu3rd.ru/cu3rd/db-access/services/provision"
	"git.cu3rd.ru/cu3rd/db-access/tests"

	"github.com/stretchr/testify/require"
)

var baseURL string

func TestMain(m *testing.M) {
	unittest.MainTest(m, func() {
		addr := os.Getenv("DBACCESS_E2E_ADDR")
		if addr == "" {
			addr = "localhost:3003"
		}
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "listen %s: %v\n", addr, err)
			os.Exit(1)
		}
		baseURL = "http://" + ln.Addr().String()
		access_service.SetProvisioner(provision.NewFake())
		mux := http.NewServeMux()
		mux.HandleFunc("POST /_test/reset-fixtures", func(w http.ResponseWriter, _ *http.Request) {
			if err := unittest.LoadFixtures(); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
		mux.Handle("/", routers.NormalRoutes())
		go func() { _ = http.Serve(ln, mux) }()
	})
}

func TestE2e(t *testing.T) {
	root := unittest.ProjectRoot()
	files, err := filepath.Glob(filepath.Join(root, "tests", "e2e", "*.test.e2e.ts"))
	require.NoError(t, err)
	require.NotEmpty(t, files)

	project := os.Getenv("PLAYWRIGHT_PROJECT")
	if project == "" {
		project = "chromium"
	}

	for _, path := range files {
		name := strings.TrimSuffix(filepath.Base(path), ".test.e2e.ts")
		t.Run(name, func(t *testing.T) {
			tests.PrepareTestEnv(t)

			args := []string{"playwright", "test", "--project=" + project, path}
			if os.Getenv("PLAYWRIGHT_UPDATE_SNAPSHOTS") != "" {
				args = append(args, "--update-snapshots")
			}
			cmd := exec.CommandContext(t.Context(), "npx", args...)
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "DBACCESS_URL="+baseURL)
			var out bytes.Buffer
			cmd.Stdout, cmd.Stderr = &out, &out
			if err := cmd.Run(); err != nil {
				t.Fatalf("playwright %s failed: %v\n%s", name, err, out.String())
			}
			t.Log(out.String())
		})
	}
}
