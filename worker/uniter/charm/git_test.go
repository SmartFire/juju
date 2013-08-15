// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package charm_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	gc "launchpad.net/gocheck"

	corecharm "launchpad.net/juju-core/charm"
	"launchpad.net/juju-core/testing"
	"launchpad.net/juju-core/testing/checkers"
	"launchpad.net/juju-core/worker/uniter/charm"
)

var curl = corecharm.MustParseURL("cs:series/blah-blah-123")

type GitDirSuite struct {
	testing.GitSuite
	LoggingSuite testing.LoggingSuite
	oldLcAll     string
}

var _ = gc.Suite(&GitDirSuite{})

func (s *GitDirSuite) SetUpTest(c *gc.C) {
	s.GitSuite.SetUpTest(c)
	s.LoggingSuite.SetUpTest(c)
	s.oldLcAll = os.Getenv("LC_ALL")
	os.Setenv("LC_ALL", "en_US")
}

func (s *GitDirSuite) TearDownTest(c *gc.C) {
	os.Setenv("LC_ALL", s.oldLcAll)
	s.LoggingSuite.TearDownTest(c)
	s.GitSuite.TearDownTest(c)
}

func (s *GitDirSuite) TestInitConfig(c *gc.C) {
	base := c.MkDir()
	repo := charm.NewGitDir(filepath.Join(base, "repo"))
	err := repo.Init()
	c.Assert(err, gc.IsNil)

	cmd := exec.Command("git", "config", "--list", "--local")
	cmd.Dir = repo.Path()
	out, err := cmd.Output()
	c.Assert(err, gc.IsNil)
	c.Assert(string(out), gc.Matches, "(.|\n)*user.email=juju@localhost.\nuser.name=juju(.|\n)*")
}

func (s *GitDirSuite) TestCreate(c *gc.C) {
	base := c.MkDir()
	repo := charm.NewGitDir(filepath.Join(base, "repo"))
	exists, err := repo.Exists()
	c.Assert(err, gc.IsNil)
	c.Assert(exists, gc.Equals, false)

	err = ioutil.WriteFile(repo.Path(), nil, 0644)
	c.Assert(err, gc.IsNil)
	_, err = repo.Exists()
	c.Assert(err, gc.ErrorMatches, `".*/repo" is not a directory`)
	err = os.Remove(repo.Path())
	c.Assert(err, gc.IsNil)

	err = os.Chmod(base, 0555)
	c.Assert(err, gc.IsNil)
	defer os.Chmod(base, 0755)
	err = repo.Init()
	c.Assert(err, gc.ErrorMatches, ".* permission denied")
	exists, err = repo.Exists()
	c.Assert(err, gc.IsNil)
	c.Assert(exists, gc.Equals, false)

	err = os.Chmod(base, 0755)
	c.Assert(err, gc.IsNil)
	err = repo.Init()
	c.Assert(err, gc.IsNil)
	exists, err = repo.Exists()
	c.Assert(err, gc.IsNil)
	c.Assert(exists, gc.Equals, true)

	_, err = charm.ReadCharmURL(repo)
	c.Assert(err, checkers.Satisfies, os.IsNotExist)

	err = repo.Init()
	c.Assert(err, gc.IsNil)
}

func (s *GitDirSuite) TestAddCommitPullRevert(c *gc.C) {
	target := charm.NewGitDir(c.MkDir())
	err := target.Init()
	c.Assert(err, gc.IsNil)
	err = ioutil.WriteFile(filepath.Join(target.Path(), "initial"), []byte("initial"), 0644)
	c.Assert(err, gc.IsNil)
	err = charm.WriteCharmURL(target, curl)
	c.Assert(err, gc.IsNil)
	err = target.AddAll()
	c.Assert(err, gc.IsNil)
	dirty, err := target.Dirty()
	c.Assert(err, gc.IsNil)
	c.Assert(dirty, gc.Equals, true)
	err = target.Commitf("initial")
	c.Assert(err, gc.IsNil)
	dirty, err = target.Dirty()
	c.Assert(err, gc.IsNil)
	c.Assert(dirty, gc.Equals, false)

	source := newRepo(c)
	err = target.Pull(source)
	c.Assert(err, gc.IsNil)
	url, err := charm.ReadCharmURL(target)
	c.Assert(err, gc.IsNil)
	c.Assert(url, gc.DeepEquals, curl)
	fi, err := os.Stat(filepath.Join(target.Path(), "some-dir"))
	c.Assert(err, gc.IsNil)
	c.Assert(fi, checkers.Satisfies, os.FileInfo.IsDir)
	data, err := ioutil.ReadFile(filepath.Join(target.Path(), "some-file"))
	c.Assert(err, gc.IsNil)
	c.Assert(string(data), gc.Equals, "hello")
	dirty, err = target.Dirty()
	c.Assert(err, gc.IsNil)
	c.Assert(dirty, gc.Equals, false)

	err = ioutil.WriteFile(filepath.Join(target.Path(), "another-file"), []byte("blah"), 0644)
	c.Assert(err, gc.IsNil)
	dirty, err = target.Dirty()
	c.Assert(err, gc.IsNil)
	c.Assert(dirty, gc.Equals, true)
	err = source.AddAll()
	c.Assert(err, gc.IsNil)
	dirty, err = target.Dirty()
	c.Assert(err, gc.IsNil)
	c.Assert(dirty, gc.Equals, true)

	err = target.Revert()
	c.Assert(err, gc.IsNil)
	_, err = os.Stat(filepath.Join(target.Path(), "some-file"))
	c.Assert(err, checkers.Satisfies, os.IsNotExist)
	_, err = os.Stat(filepath.Join(target.Path(), "some-dir"))
	c.Assert(err, checkers.Satisfies, os.IsNotExist)
	data, err = ioutil.ReadFile(filepath.Join(target.Path(), "initial"))
	c.Assert(err, gc.IsNil)
	c.Assert(string(data), gc.Equals, "initial")
	dirty, err = target.Dirty()
	c.Assert(err, gc.IsNil)
	c.Assert(dirty, gc.Equals, false)
}

func (s *GitDirSuite) TestClone(c *gc.C) {
	repo, err := newRepo(c).Clone(c.MkDir())
	c.Assert(err, gc.IsNil)
	_, err = os.Stat(filepath.Join(repo.Path(), "some-file"))
	c.Assert(err, checkers.Satisfies, os.IsNotExist)
	_, err = os.Stat(filepath.Join(repo.Path(), "some-dir"))
	c.Assert(err, checkers.Satisfies, os.IsNotExist)
	dirty, err := repo.Dirty()
	c.Assert(err, gc.IsNil)
	c.Assert(dirty, gc.Equals, true)

	err = repo.AddAll()
	c.Assert(err, gc.IsNil)
	dirty, err = repo.Dirty()
	c.Assert(err, gc.IsNil)
	c.Assert(dirty, gc.Equals, true)
	err = repo.Commitf("blank overwrite")
	c.Assert(err, gc.IsNil)
	dirty, err = repo.Dirty()
	c.Assert(err, gc.IsNil)
	c.Assert(dirty, gc.Equals, false)

	lines, err := repo.Log()
	c.Assert(err, gc.IsNil)
	c.Assert(lines, gc.HasLen, 2)
	c.Assert(lines[0], gc.Matches, "[a-f0-9]{7} blank overwrite")
	c.Assert(lines[1], gc.Matches, "[a-f0-9]{7} im in ur repo committin ur files")
}

func (s *GitDirSuite) TestConflictRevert(c *gc.C) {
	source := newRepo(c)
	updated, err := source.Clone(c.MkDir())
	c.Assert(err, gc.IsNil)
	err = ioutil.WriteFile(filepath.Join(updated.Path(), "some-dir"), []byte("hello"), 0644)
	c.Assert(err, gc.IsNil)
	err = updated.Snapshotf("potential conflict src")
	c.Assert(err, gc.IsNil)
	conflicted, err := updated.Conflicted()
	c.Assert(err, gc.IsNil)
	c.Assert(conflicted, gc.Equals, false)

	target := charm.NewGitDir(c.MkDir())
	err = target.Init()
	c.Assert(err, gc.IsNil)
	err = target.Pull(source)
	c.Assert(err, gc.IsNil)
	err = ioutil.WriteFile(filepath.Join(target.Path(), "some-dir", "conflicting-file"), []byte("hello"), 0644)
	c.Assert(err, gc.IsNil)
	err = target.Snapshotf("potential conflict dst")
	c.Assert(err, gc.IsNil)
	conflicted, err = target.Conflicted()
	c.Assert(err, gc.IsNil)
	c.Assert(conflicted, gc.Equals, false)

	err = target.Pull(updated)
	c.Assert(err, gc.Equals, charm.ErrConflict)
	conflicted, err = target.Conflicted()
	c.Assert(err, gc.IsNil)
	c.Assert(conflicted, gc.Equals, true)
	dirty, err := target.Dirty()
	c.Assert(err, gc.IsNil)
	c.Assert(dirty, gc.Equals, true)

	err = target.Revert()
	c.Assert(err, gc.IsNil)
	conflicted, err = target.Conflicted()
	c.Assert(err, gc.IsNil)
	c.Assert(conflicted, gc.Equals, false)
	dirty, err = target.Dirty()
	c.Assert(err, gc.IsNil)
	c.Assert(dirty, gc.Equals, false)
}

func newRepo(c *gc.C) *charm.GitDir {
	repo := charm.NewGitDir(c.MkDir())
	err := repo.Init()
	c.Assert(err, gc.IsNil)
	err = os.Mkdir(filepath.Join(repo.Path(), "some-dir"), 0755)
	c.Assert(err, gc.IsNil)
	err = ioutil.WriteFile(filepath.Join(repo.Path(), "some-file"), []byte("hello"), 0644)
	c.Assert(err, gc.IsNil)
	err = repo.AddAll()
	c.Assert(err, gc.IsNil)
	err = repo.Commitf("im in ur repo committin ur %s", "files")
	c.Assert(err, gc.IsNil)
	return repo
}
