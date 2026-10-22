//go:build ignore

// One-off: run an install job and poll until it finishes. Usage:
//   go run scripts/run_install_job.go irgipaulius/rampocalypse
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/irgipaulius/cogs/pkg/installjob"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

func main() {
	repo := "irgipaulius/rampocalypse"
	if len(os.Args) > 1 {
		repo = os.Args[1]
	}
	wsRoot, _, err := workspace.FindRoot(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	m := installjob.NewManager(wsRoot)
	job, err := m.Create(context.Background(), installjob.Request{
		RepoFull: repo,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("install_id:", job.ID)
	for {
		time.Sleep(5 * time.Second)
		j, err := m.Get(job.ID)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		cur, tot := j.CurrentStage, len(installjob.StageOrder)
		fmt.Printf("[%s] stage=%s (%s) err=%q\n", j.Status, cur, j.Stages, j.Error)
		_ = tot
		if j.Status == installjob.StatusSucceeded || j.Status == installjob.StatusFailed {
			b, _ := json.MarshalIndent(j, "", "  ")
			fmt.Println(string(b))
			if j.Status == installjob.StatusFailed {
				tail, _ := m.LogTail(j.ID, 64*1024)
				fmt.Println("--- log tail ---")
				fmt.Println(tail)
				os.Exit(1)
			}
			return
		}
	}
}
