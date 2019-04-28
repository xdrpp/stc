package main

import "os"
import "os/exec"
import "path/filepath"
import "testing"

func TestCompile(t *testing.T) {
	goxdr := "./goxdr"			// Can't join "."
	source := filepath.Join("testdata", "testxdr.x")
	source_strict := filepath.Join("testdata", "testxdr_strict.x")
	target := filepath.Join("testdata", "testxdr.go")

	cmd := exec.Command(goxdr, "-enum-comments", "-p", "testxdr",
		"-o", target, source)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("goxdr failed:\n%s", out)
		return
	}
	cmd = exec.Command("go", "build", target)
	if _, err := cmd.CombinedOutput(); err == nil {
		t.Errorf("goxdr's failed to detect lax unions")
		return
	}

	cmd = exec.Command(goxdr, "-enum-comments", "-p", "testxdr",
		"-o", target, source_strict)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("goxdr failed:\n%s", out)
		return
	}
	cmd = exec.Command("go", "build", target)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("goxdr's output without -lax-discriminants " +
			"failed to compile:\n%s", out)
		return
	}

	cmd = exec.Command(goxdr, "-lax-discriminants", "-enum-comments",
		"-p", "testxdr", "-o", target, source)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("goxdr failed:\n%s", out)
		return
	}
	cmd = exec.Command("go", "build", target)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("goxdr's output failed to compile:\n%s", out)
		return
	}
	os.Remove(target)
}
