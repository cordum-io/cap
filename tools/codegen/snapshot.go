package codegen

import (
	"io/fs"
	"path"
	"sort"
)

// GeneratorRule maps a generator to the output globs it owns. A file is
// attributed to the first rule whose glob matches it.
type GeneratorRule struct {
	ID    string
	Globs []string
}

// Snapshot walks root and produces a Manifest describing exactly the generated
// files that match the rules, pinned by sha256. It is how the checked-in
// baseline manifest is (re)built after a legitimate regeneration; Check then
// holds the tree to it. Snapshot only records files a rule claims, so a
// hand-written file outside every glob is never blessed as generated.
func Snapshot(root fs.FS, sources []string, rules []GeneratorRule) (*Manifest, error) {
	m := &Manifest{Version: 1, Sources: append([]string(nil), sources...)}
	genOutputs := map[string][]string{}
	err := fs.WalkDir(root, ".", func(name string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		rule, ok := matchRule(name, rules)
		if !ok {
			return nil
		}
		data, err := fs.ReadFile(root, name)
		if err != nil {
			return err
		}
		m.Outputs = append(m.Outputs, Output{Path: name, SHA256: hashContent(data)})
		genOutputs[rule] = append(genOutputs[rule], name)
		return nil
	})
	if err != nil {
		return nil, err
	}
	finalizeManifest(m, rules, genOutputs)
	return m, nil
}

func matchRule(name string, rules []GeneratorRule) (string, bool) {
	for _, r := range rules {
		for _, g := range r.Globs {
			if ok, _ := path.Match(g, name); ok {
				return r.ID, true
			}
		}
	}
	return "", false
}

func finalizeManifest(m *Manifest, rules []GeneratorRule, genOutputs map[string][]string) {
	sort.Slice(m.Outputs, func(i, j int) bool { return m.Outputs[i].Path < m.Outputs[j].Path })
	globs := map[string]bool{}
	for _, r := range rules {
		for _, g := range r.Globs {
			globs[g] = true
		}
	}
	m.GeneratedGlobs = sortedSet(globs)
	ids := make([]string, 0, len(genOutputs))
	for id := range genOutputs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		outs := genOutputs[id]
		sort.Strings(outs)
		m.Generators = append(m.Generators, Generator{ID: id, Outputs: outs})
	}
}

func sortedSet(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
