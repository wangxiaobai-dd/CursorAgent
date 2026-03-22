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
	return &SvnDiffSource{opt: opt, workDir: diffDir, metaMap: make(map[string]string)}
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
	if rev, ok := item.Meta["revision"]; ok {
		msg := firstLineCommitMsg(item.Meta["msg"])
		return "REVISION:" + rev + "\t\t" + msg
	}
	base := filepath.Base(item.Path)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	rev, msg := util.ParseRevisionFromDiffFilename(base)
	msg = firstLineCommitMsg(msg)
	return "REVISION:" + rev + "\t\t" + msg
}

func pluginDiffPromptBlock(skillName, diffPath, resultFile, header, sourceRefs string) string {
	var b strings.Builder
	b.WriteString("请使用 skill 文件 ")
	b.WriteString(skillName)
	b.WriteString(" 检查 @")
	b.WriteString(diffPath)
	b.WriteString("，只输出结果，不要其他操作。")
	b.WriteString(sourceRefs)
	b.WriteString("\n\n结果追加写入文件：")
	b.WriteString(resultFile)
	b.WriteString("，写入头部为：")
	b.WriteString(header)
	return b.String()
}

func (p *SvnDiffSource) BuildMergedDiffReviewPrompt(skillName, resultFileAbs string, items []types.InputItem) string {
	n := len(items)
	var b strings.Builder
	if n > 1 {
		b.WriteString("以下共 ")
		b.WriteString(strconv.Itoa(n))
		b.WriteString(" 段审查，请按顺序完成；审查结果按 skill 要求分 Revision 块，必须全部追加写入到结果文件（唯一目标路径，禁止其它文件名如工作区目录名+.txt）：\n")
		b.WriteString(resultFileAbs)
		b.WriteString("\n\n")
	}
	for i, item := range items {
		if i > 0 {
			b.WriteString("\n\n")
		}
		absDiff, _ := filepath.Abs(item.Path)
		header := p.revisionHeaderForItem(item)
		var sourceRefs string
		if p.opt.IncludeSvnSource && strings.TrimSpace(p.opt.ProjectPath) != "" {
			if refs, err := util.BuildSourceRefsFromDiff(absDiff, p.opt.ProjectPath); err != nil {
				log.Printf("svn_diff: BuildSourceRefsFromDiff %s: %v", absDiff, err)
			} else {
				sourceRefs = refs
			}
		}
		b.WriteString(pluginDiffPromptBlock(skillName, absDiff, resultFileAbs, header, sourceRefs))
	}
	return strings.TrimSpace(b.String())
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
		meta := map[string]string{"revision": rev, "msg": firstLineCommitMsg(p.metaMap[rev])}
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
