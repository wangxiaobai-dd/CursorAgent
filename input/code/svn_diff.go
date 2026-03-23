package code

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"CursorAgent/config"
	"CursorAgent/input/types"
	"CursorAgent/util"
)

type SvnDiffSource struct {
	opt       *config.TaskInput
	workDir   string
	diffFiles []string
	metaMap   map[string]string // revision -> commit msg
	authorMap map[string]string // revision -> svn log author
	mu        sync.Mutex
}

func NewSvnDiffSource(opt *config.TaskInput) *SvnDiffSource {
	base := opt.DiffDir
	if base == "" {
		base = "."
	}
	today := time.Now().Format("2006-01-02")
	diffDir := filepath.Join(base, today)
	if err := os.MkdirAll(diffDir, 0755); err != nil {
		log.Printf("svn_diff: mkdir %s failed: %v", diffDir, err)
	}
	return &SvnDiffSource{opt: opt, workDir: diffDir, metaMap: make(map[string]string), authorMap: make(map[string]string)}
}

func (p *SvnDiffSource) WorkDir() string { return p.workDir }

func firstLineCommitMsg(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s
}

func (p *SvnDiffSource) revisionHeaderForItem(item types.InputItem) string {
	var rev, msg, author string
	if r, ok := item.Meta["revision"]; ok {
		rev = r
		msg = firstLineCommitMsg(item.Meta["msg"])
		author = strings.TrimSpace(item.Meta["author"])
	} else {
		base := filepath.Base(item.Path)
		base = strings.TrimSuffix(base, filepath.Ext(base))
		parsedRev, parsedMsg := util.ParseRevisionFromDiffFilename(base)
		rev = parsedRev
		msg = firstLineCommitMsg(parsedMsg)
		author = strings.TrimSpace(p.authorMap[rev])
		if msg == "" {
			msg = firstLineCommitMsg(p.metaMap[rev])
		}
	}
	if author == "" {
		author = strings.TrimSpace(p.authorMap[rev])
	}
	// 头部为单行：REVISION + 提交说明 + AUTHOR。
	return "REVISION:" + rev + "\t\t" + msg + "   AUTHOR:" + author
}

// RevisionHeaderForItem 供 runner 侧写入结果文件头部使用。
func (p *SvnDiffSource) RevisionHeaderForItem(item types.InputItem) string {
	return p.revisionHeaderForItem(item)
}

type argsMode int

const (
	addRepoURL argsMode = iota
	addProjectPath
)

func (p *SvnDiffSource) buildArgs(mode argsMode, args ...string) []string {
	cmdArgs := append(args, "--username", p.opt.UserName, "--password", p.opt.Password)
	switch mode {
	case addRepoURL:
		cmdArgs = append(cmdArgs, p.opt.RepoURL)
	case addProjectPath:
		cmdArgs = append(cmdArgs, p.opt.ProjectPath)
	}
	return cmdArgs
}

func (p *SvnDiffSource) execSvn(mode argsMode, args ...string) (string, error) {
	cmdArgs := p.buildArgs(mode, args...)
	cmd := exec.Command("svn", cmdArgs...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	log.Printf("execSvn: %v", cmd.Args)
	err := cmd.Run()
	return out.String(), err
}

type svnLog struct {
	Entries []struct {
		Revision string `xml:"revision,attr"`
		Author   string `xml:"author"`
		Msg      string `xml:"msg"`
	} `xml:"logentry"`
}

func (p *SvnDiffSource) getRevisions() ([]string, error) {
	end := time.Now()
	start := end.AddDate(0, 0, p.opt.CheckDay)
	endDate := end.Format("2006-01-02")
	startDate := start.Format("2006-01-02")
	if p.opt.CheckDay > 0 {
		endDate, startDate = startDate, endDate
	}
	out, err := p.execSvn(addRepoURL, "log", "-r", "{"+startDate+"}:{"+endDate+"}", "--xml")
	if err != nil {
		return nil, err
	}
	var logData svnLog
	if err := xml.Unmarshal([]byte(out), &logData); err != nil {
		return nil, fmt.Errorf("parse svn log: %v", err)
	}
	var revs []string
	for _, e := range logData.Entries {
		revs = append(revs, e.Revision)
		p.metaMap[e.Revision] = e.Msg
		p.authorMap[e.Revision] = strings.TrimSpace(e.Author)
	}
	return revs, nil
}

func (p *SvnDiffSource) prevRev(rev string) string {
	n, _ := strconv.Atoi(rev)
	return strconv.Itoa(n - 1)
}

func (p *SvnDiffSource) diffPath(rev string) string {
	msgPart := sanitizeFileNamePart(p.metaMap[rev])
	return filepath.Join(p.workDir, rev+"_"+msgPart+".diff")
}

func sanitizeFileNamePart(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "no_msg"
	}
	var b strings.Builder
	lastUnderscore := false
	for _, r := range s {
		switch {
		case r < 32 || strings.ContainsRune(`<>:"/\|?*`, r):
			if !lastUnderscore {
				b.WriteByte('-')
				lastUnderscore = true
			}
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			if !lastUnderscore {
				b.WriteByte('-')
				lastUnderscore = true
			}
		default:
			b.WriteRune(r)
			lastUnderscore = false
		}
	}
	out := strings.Trim(b.String(), " ._")
	if out == "" {
		out = "no_msg"
	}
	runes := []rune(out)
	if len(runes) > 60 {
		out = string(runes[:60])
	}
	return out
}

func (p *SvnDiffSource) GetInputs() ([]types.InputItem, error) {
	p.mu.Lock()
	p.diffFiles = nil
	p.mu.Unlock()

	if err := util.ClearDir(p.workDir); err != nil {
		log.Printf("clear diff dir failed: %v", err)
		return nil, err
	}

	if _, err := p.execSvn(addProjectPath, "update"); err != nil {
		log.Printf("svn update failed: %v", err)
		return nil, err
	}
	revisions, err := p.getRevisions()
	if err != nil {
		return nil, err
	}
	log.Printf("revisions: %v", revisions)

	var wg sync.WaitGroup
	for _, rev := range revisions {
		prev := p.prevRev(rev)
		out, err := p.execSvn(addRepoURL, "diff", "-r", prev+":"+rev)
		if err != nil {
			log.Printf("svn diff -r %s:%s failed: %v", prev, rev, err)
			continue
		}
		wg.Add(1)
		go func(rev, out string) {
			defer wg.Done()
			path := p.diffPath(rev)
			util.WriteContentToFile(out, path)
			p.mu.Lock()
			p.diffFiles = append(p.diffFiles, path)
			p.mu.Unlock()
		}(rev, out)
	}
	wg.Wait()
	log.Printf("svn diff files: %v", p.diffFiles)

	items := make([]types.InputItem, 0, len(p.diffFiles))
	for _, f := range p.diffFiles {
		base := filepath.Base(f)
		name := strings.TrimSuffix(base, filepath.Ext(base))
		rev := name
		if idx := strings.Index(name, "_"); idx > 0 {
			rev = name[:idx]
		}
		meta := map[string]string{
			"revision": rev,
			"msg":      firstLineCommitMsg(p.metaMap[rev]),
			"author":   p.authorMap[rev],
		}
		items = append(items, types.InputItem{Path: f, Meta: meta})
	}
	return items, nil
}

// Cleanup removes generated diff files after job run.
func (p *SvnDiffSource) Cleanup() {
	// p.mu.Lock()
	// files := append([]string(nil), p.diffFiles...)
	// p.mu.Unlock()
	// for _, f := range files {
	// 	if err := os.Remove(f); err != nil {
	// 		log.Printf("failed to remove diff file %s: %v", f, err)
	// 	}
	// }
}
