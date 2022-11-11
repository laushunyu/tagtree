package main

import (
	"errors"
	"fmt"
	"github.com/blushft/go-diagrams/diagram"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"log"
	"os"
	"regexp"
	"runtime/debug"
	"strings"
)

var (
	repoPath = `/home/macoo/chaitin/cloudwalker`
)

func main() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err, string(debug.Stack()))
		}
	}()

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		panic(err)
	}

	os.RemoveAll("go-diagrams")

	tree, err := diagram.New()
	if err != nil {
		panic(err)
	}

	idGen := func(major, minor, patch, release, pre string) string {
		i := fmt.Sprintf("%s.%s.%s", major, minor, patch)
		if release != "" {
			i += "-"
			i += release
		}
		if pre != "" {
			i += "-"
			i += pre
		}
		return i
	}

	existed := map[string]struct{}{}
	nodeIdIndex := map[string]string{}

	if err := RecursiveRepoTag(repo, func(tag, major, minor, patch, release, pre string) {
		if _, ok := existed[major+minor]; !ok {
			name := idGen(major, minor, "", "", "")
			node := NewNode(name, "", "")
			tree.Add(node)
			nodeIdIndex[name] = node.ID()
		}
		if _, ok := existed[major+minor+patch]; !ok {
			name := idGen(major, minor, patch, "", "")
			node := NewNode(name, "", "")
			tree.Add(node)
			nodeIdIndex[name] = node.ID()
		}
		if _, ok := existed[major+minor+patch+release]; !ok {
			name := idGen(major, minor, patch, release, "")
			node := NewNode(name, "", "")
			tree.Add(node)
			nodeIdIndex[name] = node.ID()
		}

		name := idGen(major, minor, patch, release, pre)
		node := NewNode(name, "", "")
		tree.Add(node)
		nodeIdIndex[name] = node.ID()
	}); err != nil {
		panic(err)
	}

	lineIndex := map[[2]string]struct{}{}

	if err := RecursiveRepoTag(repo, func(tag, major, minor, patch, release, pre string) {
		name := idGen(major, minor, patch, release, pre)

		var parent string
		if pre != "" {
			parent = idGen(major, minor, patch, release, "")
			if _, ok := lineIndex[[2]string{parent, name}]; !ok {
				tree.ConnectByID(nodeIdIndex[parent], nodeIdIndex[name])
				lineIndex[[2]string{parent, name}] = struct{}{}
			}
		}
		if release != "" {
			parent = idGen(major, minor, patch, "", "")
			if _, ok := lineIndex[[2]string{parent, name}]; !ok {
				tree.ConnectByID(nodeIdIndex[parent], nodeIdIndex[name])
				lineIndex[[2]string{parent, name}] = struct{}{}
			}
		}

		parent = idGen(major, minor, "", "", "")
		if _, ok := lineIndex[[2]string{parent, name}]; !ok {
			tree.ConnectByID(nodeIdIndex[parent], nodeIdIndex[name])
			lineIndex[[2]string{parent, name}] = struct{}{}
		}
	}); err != nil {
		panic(err)
	}

	fmt.Println(tree.Edges())

	if err := tree.Render(); err != nil {
		panic(err)
	}
}

func NewNode(name, assigner, comment string) *diagram.Node {
	return diagram.NewNode(
		diagram.Name("node"),
		diagram.Provider(assigner),
		diagram.NodeLabel(strings.ReplaceAll(name, ".", "-")),
		diagram.FixedSize(false),
	)
}

type Tag struct {
	*object.Commit
	Name string
}

var refIndex = make(map[plumbing.Hash]*plumbing.Reference)

var tagPattern = regexp.MustCompile(`(?i)^CW-[CS]10-(\d+).(\d+).(\d+)(?:[-_]([a-z]+))?(?:[-_]((?:r|rc|p)\d+))?$`)

func RecursiveRepoTag(repo *git.Repository, fn func(tag, major, minor, patch, release, pre string)) error {
	// collect all tags and its hash
	tags, err := repo.Tags()
	if err != nil {
		return err
	}
	defer tags.Close()
	if err := tags.ForEach(func(ref *plumbing.Reference) error {
		refIndex[ref.Hash()] = ref
		tagName := ref.Name().Short()

		sub := tagPattern.FindStringSubmatch(tagName)
		if len(sub) == 0 {
			fmt.Println("unsupported tag", tagName)
			return nil
		}

		tag, major, minor, patch, release, pre := sub[0], sub[1], sub[2], sub[3], sub[4], sub[5]
		fn(tag, major, minor, patch, release, pre)
		return nil
	}); err != nil {
		return err
	}
	return nil

	//// recursive log tree
	//logTree, err := repo.Log(&git.LogOptions{
	//	All: true,
	//})
	//defer logTree.Close()
	//if err := logTree.ForEach(func(commit *object.Commit) error {
	//	self, ok := refIndex[commit.ID()]
	//	if !ok {
	//		return nil
	//	}
	//
	//	commit.Stats()
	//
	//	fmt.Println(self.Name(), "has", commit.NumParents(), "parent")
	//
	//	if !recursiveParent(commit, func(parentCommit *object.Commit) (found bool) {
	//		parent, ok := refIndex[parentCommit.ID()]
	//		if !ok {
	//			return false
	//		}
	//
	//		fn(&Tag{Commit: parentCommit, Name: parent.Name().Short()}, &Tag{Commit: commit, Name: self.Name().Short()})
	//		return true
	//	}) {
	//		fn(nil, &Tag{Commit: commit, Name: self.Name().Short()})
	//	}
	//	return nil
	//}); err != nil {
	//	return err
	//}

	return nil
}

func recursiveParent(commit *object.Commit, fn func(commit *object.Commit) (found bool)) (found bool) {
	parents := commit.Parents()
	for {
		parentCommit, err := parents.Next()
		if err != nil {
			if errors.Is(err, storer.ErrStop) {
				return false
			}
			log.Print(fmt.Errorf("failed to recursive parent %s commit: %s", commit.Hash, err))
			return false
		}

		if fn(parentCommit) {
			return true
		}

		if recursiveParent(parentCommit, fn) {
			return true
		}
	}
}
