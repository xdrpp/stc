package main

import "os"
import "os/exec"
import "path/filepath"
import "testing"

func TestCompile(t *testing.T) {
	goxdr := "./goxdr"			// Can't join "."
	source := filepath.Join("testdata", "testxdr.x")
	target := filepath.Join("testdata", "testxdr.go")
	cmd := exec.Command(goxdr, "-enum-comments", "-p", "testxdr",
		"-o", target, source)
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
