package proxy_test

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"testing"

	"github.com/jcbhmr/xmod/proxy"
	"golang.org/x/mod/modfile"
)

type HTTPClientOps struct {
	BaseURL string
}

func (h *HTTPClientOps) ReadRemote(p string) ([]byte, error) {
	resp, err := http.Get(h.BaseURL + p)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s %d", resp.Request.URL, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func (h *HTTPClientOps) Log(msg string) {
	log.Print(msg)
}

func ExampleClient() {
	client := proxy.NewClient(&HTTPClientOps{
		BaseURL: "https://proxy.golang.org",
	})

	modulePath := "golang.org/x/mod"

	repo, err := client.Lookup(modulePath)
	if err != nil {
		log.Fatal(err)
	}

	versions, err := repo.Versions("")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(versions)
	// Possible output: [v0.24.0 v0.23.0 v0.22.0 ...]

	latest, err := repo.Latest()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(latest.Version)
	// Possible output: v0.24.0

	goMod, err := repo.GoMod(latest.Version)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(goMod))
	// Possible output: 1234

	buffer := &bytes.Buffer{}
	err = repo.Zip(buffer, latest.Version)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(buffer.Len())
	// Possible output: 123456
}

func TestClient(t *testing.T) {
	client := proxy.NewClient(&HTTPClientOps{
		BaseURL: "https://proxy.golang.org",
	})

	modulePath := "golang.org/x/mod"

	repo, err := client.Lookup(modulePath)
	if err != nil {
		t.Fatal(err)
	}

	versions, err := repo.Versions("")
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) == 0 {
		t.Fatal("expected non-empty version list")
	}

	latest, err := repo.Latest()
	if err != nil {
		t.Fatal(err)
	}
	if latest.Version == "" {
		t.Fatal("expected non-empty latest version")
	}

	goMod, err := repo.GoMod(latest.Version)
	if err != nil {
		t.Fatal(err)
	}
	_, err = modfile.Parse(modulePath+"@"+latest.Version+" go.mod", goMod, nil)
	if err != nil {
		t.Fatal(err)
	}

	buffer := &bytes.Buffer{}
	err = repo.Zip(buffer, latest.Version)
	if err != nil {
		t.Fatal(err)
	}
	zipr, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(zipr.File) == 0 {
		t.Fatal("expected non-empty zip file")
	}
}
