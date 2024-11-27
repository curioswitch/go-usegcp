package main

import (
	"archive/tar"
	"bytes"
	"embed"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/curioswitch/go-build"
	"github.com/goyek/goyek/v2"
	"github.com/goyek/x/boot"
	"github.com/goyek/x/cmd"
)

//go:embed otel
var otelFS embed.FS

//go:embed version-otel-collector.txt
var verOTelCollectorContrib string

func main() {
	build.DefineTasks()

	dockerTags := flag.String("docker-tags", "ghcr.io/curioswitch/go-usegcp/otel-collector:dev", "Tags to add to built docker image.")
	dockerPush := flag.Bool("docker-push", false, "Whether to push built docker images")

	goyek.Define(goyek.Task{
		Name: "otel-collector",
		Action: func(a *goyek.A) {
			if err := os.MkdirAll("out", 0o755); err != nil {
				a.Error(err)
			}

			otelfsPath := filepath.Join("out", "otelfs.tar")
			func() {
				f, err := os.Create(otelfsPath)
				if err != nil {
					a.Error(err)
				}
				defer f.Close()

				w := tar.NewWriter(f)
				defer w.Close()

				if err := w.AddFS(otelFS); err != nil {
					a.Error(err)
				}
			}()

			tags := strings.Split(*dockerTags, ",")
			baseTag := tags[0]
			repo, _, _ := strings.Cut(baseTag, ":")

			var digests []string
			for _, platform := range []string{"linux/amd64", "linux/arm64"} {
				var stdout bytes.Buffer

				mutateCmd := fmt.Sprintf(
					"go run github.com/google/go-containerregistry/cmd/crane@%s --platform %s mutate --append %s otel/opentelemetry-collector-contrib:%s",
					verCrane, platform, otelfsPath, verOTelCollectorContrib)
				if *dockerPush {
					mutateCmd += " --repo " + repo
				} else {
					mutateCmd += " --output " + filepath.Join("out", fmt.Sprintf("collector-%s.tar", strings.ReplaceAll(platform, "/", "_")))
				}
				cmd.Exec(a, mutateCmd, cmd.Stdout(&stdout))

				digests = append(digests, stdout.String())
			}

			if *dockerPush {
				indexCmd := fmt.Sprintf(
					"go run github.com/google/go-containerregistry/cmd/crane@%s index append -t %s",
					verCrane, baseTag)

				for _, digest := range digests {
					indexCmd += " -m " + digest
				}

				cmd.Exec(a, indexCmd)

				for _, tag := range tags[1:] {
					_, tag, _ := strings.Cut(tag, ":")
					cmd.Exec(a, fmt.Sprintf("go run github.com/google/go-containerregistry/cmd/crane@%s tag %s %s", verCrane, baseTag, tag))
				}
			}
		},
	})

	boot.Main()
}
