// +build ini

// This program is just for testing the ini parser/editor in the stc
// library.  You don't really need this program, because git-config
// does the same thing.
package main

import "fmt"
import "github.com/xdrpp/stc/stcdetail"
import "os"
import "path"
import "strings"

var progname = "ini"

func usage(n int) {
	out := os.Stderr
	if n == 0 {
		out = os.Stdout
	}
	fmt.Fprintf(out, "usage: %s FILE {-d KEY | {-s|-a} KEY VALUE}...\n",
		progname)
	os.Exit(n)
}

func getSecKey(arg string) (sec *stcdetail.IniSection, key string) {
	if n := strings.IndexByte(arg, '.'); n >= 0 {
		sec = &stcdetail.IniSection{
			Section: arg[:n],
		}
		arg = arg[n+1:]
	}
	if n := strings.LastIndexByte(arg, '.'); n >= 0 {
		subsec := arg[:n]
		arg = arg[n+1:]
		sec.Subsection = &subsec
	}
	key = arg
	return
}

func doupdates(target string, actions []func(*stcdetail.IniEdit)) int {
	lf, err := stcdetail.LockFile(target, 0666)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer lf.Abort()

	contents, err := lf.ReadFile()
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	ie, err := stcdetail.NewIniEdit(target, contents)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, f := range actions {
		f(ie)
	}

	ie.WriteTo(lf)
	err = lf.Commit()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func main() {
	av := os.Args
	if len(av) > 0 {
		progname = path.Base(av[0])
		av = av[1:]
	}
	var actions []func(*stcdetail.IniEdit)
	if len(av) == 0 {
		usage(1)
	}
	target := av[0]
	av = av[1:]
	if target == "" || target[0] == '-' {
		if target == "-h" || target == "-help" || target == "--help" {
			usage(0)
		} else {
			usage(1)
		}
	}

	for len(av) >= 2 {
		sec, key := getSecKey(av[1])
		if sec == nil {
			fmt.Fprintf(os.Stderr, "%s: bad section.key argument %q\n",
				progname, av[1])
			os.Exit(1)
		}
		switch av[0] {
		case "-d":
			if len(av) < 2 {
				usage(1)
			}
			actions = append(actions, func(ie *stcdetail.IniEdit) {
				ie.Del(*sec, key)
			})
			av = av[2:]
		case "-s":
			if len(av) < 3 {
				usage(1)
			}
			val := av[2]
			actions = append(actions, func(ie *stcdetail.IniEdit) {
				ie.Set(*sec, key, val)
			})
			av = av[3:]
		case "-a":
			if len(av) < 3 {
				usage(1)
			}
			val := av[2]
			actions = append(actions, func(ie *stcdetail.IniEdit) {
				ie.Add(*sec, key, val)
			})
			av = av[3:]
		default:
			usage(1)
		}
	}

	if len(av) != 0 {
		usage(1)
	}
	os.Exit(doupdates(target, actions))
}
