package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

type appArgs struct {
	local    string
	prefixes []string

	listOnly bool
	write    bool
	diffOnly bool
}

func init() {
	// Configurer le logger pour inclure la date, l'heure, et les informations sur le fichier/ligne
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(os.Stdout)
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}
}

func run() error {
	var args appArgs

	flag.Usage = func() {
		_, _ = fmt.Fprintln(flag.CommandLine.Output(), "Usage: goia [flags] [path ...]")
		flag.PrintDefaults()
		os.Exit(2)
	}

	flag.StringVar(&args.local, "local", "", "put imports beginning with this string after 3rd-party package")
	flag.Var((*ArrayStringFlag)(&args.prefixes), "prefix", "relative local prefix to from a new import group (can be given several times)")

	flag.BoolVar(&args.listOnly, "l", false, "list files whose formatting differs from goia's")
	flag.BoolVar(&args.write, "w", false, "write result to (source) file instead of stdout")
	flag.BoolVar(&args.diffOnly, "d", false, "display diffs instead of rewriting files")

	flag.Parse()

	return process(&args, flag.Args()...)
}

func process(args *appArgs, paths ...string) error {
	cache := NewConfigCache(args.local, args.prefixes)

	j, err := newJob(cache, ".", args)
	if err != nil {
		return err
	}

	if len(paths) == 0 {
		j.fileName = ""
		j.source = fileSourceStdin
		return j.run()
	}

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("erreur lors de l'accès au chemin %s : %w", path, err)
		}

		if info.IsDir() {
			// L'utilisateur a sélectionné un répertoire
			fmt.Printf("traitement du répertoire : %s\n", path)
			j.fileDir = path
			if err := j.processFileFromFolder(); err != nil {
				return err
			}
		} else {
			// L'utilisateur a sélectionné un fichier
			fmt.Printf("Traitement du fichier : %s\n", path)
			j.source = fileSourceFilePath
			if err := j.run(); err != nil {
				return err
			}
		}
	}

	return nil
}

func (j *job) processFileFromFolder() error {
	filesFound, err := j.loadFilesFromFolder()
	if err != nil {
		fmt.Println(j.t("No files found in the specified folder, create a new one"), err)
		if err := j.promptNoFilesFoundCreateANewFile(); err != nil {
			return err
		}
	} else {
		if err := j.promptSelectAFileOrCreateANewOne(filesFound); err != nil {
			return err
		}
	}

	j.source = fileSourceFilePath
	if err := j.run(); err != nil {
		return err
	}
	return nil
}

func diff(b1, b2 []byte, filename string) ([]byte, error) {
	f1, err := writeTempFile("", "goia", b1)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = os.Remove(f1)
	}()

	f2, err := writeTempFile("", "goia", b2)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = os.Remove(f2)
	}()

	data, err := exec.Command("diff", "-u", f1, f2).CombinedOutput()
	if len(data) >= 0 {
		data, err = replaceTempFilename(data, filename)
	}
	return data, err
}

func writeTempFile(dir string, prefix string, data []byte) (string, error) {
	f, err := os.CreateTemp(dir, prefix)
	if err != nil {
		return "", err
	}

	_, err = f.Write(data)
	if err1 := f.Close(); err == nil {
		err = err1
	}

	if err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}

	return f.Name(), nil
}

// replaceTempFilename replaces temporary filenames in diff with actual one.
//
// --- /tmp/gofmt316145376	2017-02-03 19:13:00.280468375 -0500
// +++ /tmp/gofmt617882815	2017-02-03 19:13:00.280468375 -0500
// ...
// ->
// diff -u path/to/file.go.orig path/to/file.go
// --- path/to/file.go.orig	2017-02-03 19:13:00.280468375 -0500
// +++ path/to/file.go	2017-02-03 19:13:00.280468375 -0500
// ...
func replaceTempFilename(diff []byte, filename string) ([]byte, error) {
	bs := bytes.SplitN(diff, []byte{'\n'}, 3)
	if len(bs) < 3 {
		return nil, fmt.Errorf("got unexpected diff for %s", filename)
	}

	// Preserve timestamps.
	var t0, t1 []byte
	if i := bytes.LastIndexByte(bs[0], '\t'); i != -1 {
		t0 = bs[0][i:]
	}
	if i := bytes.LastIndexByte(bs[1], '\t'); i != -1 {
		t1 = bs[1][i:]
	}

	// Always print filepath with slash separator.
	f := filepath.ToSlash(filename)
	bs[0] = []byte(fmt.Sprintf("--- %s%s", f+".orig", t0))
	bs[1] = []byte(fmt.Sprintf("+++ %s%s", f, t1))

	// Insert diff header.
	header := fmt.Sprintf("diff -u %s %s", f+".orig", f)
	bs = append([][]byte{[]byte(header)}, bs...)

	return bytes.Join(bs, []byte{'\n'}), nil
}
