package metadata_test

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/birkland/ocfl/metadata"
	"github.com/go-test/deep"
)

var testInventory = metadata.Inventory{
	ID:              "test://myOcflObject",
	DigestAlgorithm: "sha512",
	Head:            "v2",
	Type:            "Object",
	Manifest: metadata.Manifest{
		"a": {"v1/content/physical/1", "v3/content/physical/1"},
		"b": {"v2/content/physical/2"},
		"c": {"v2/content/physical/3"},
	},
	Versions: map[string]metadata.Version{
		"v1": {
			Created: time.Now(),
			User: metadata.User{
				Name:    "孔子",
				Address: "⌖",
			},
			Message: "Hello",
			State: metadata.Manifest{
				"a": {"logical/1"},
				"b": {"logical/2"},
			},
		},
		"v2": {
			Created: time.Now(),
			User: metadata.User{
				Name:    "Khổng Tử",
				Address: "Here",
			},
			Message: "Wait",
			State: metadata.Manifest{
				"a": {"logical/1"},
				"c": {"logical/3"},
			},
		},
		"v3": {
			Created: time.Now(),
			User: metadata.User{
				Name:    "Third Editor",
				Address: "There",
			},
			Message: "Goodbye",
			State: metadata.Manifest{
				"b": {"logical/1"},
				"c": {"logical/2", "logical/2.copy"},
			},
		},
	},
	Fixity: metadata.Fixity{
		"sha256": {
			"aa": {"v1/content/physical/1", "v3/content/physical/1"},
			"bb": {"v2/content/physical/2"},
		},
	},
}

func TestParseRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	reader := bufio.NewReader(&buf)

	err := testInventory.Serialize(writer)
	if err != nil {
		t.Error(err)
	}

	writer.Flush()

	deserialized := metadata.Inventory{}
	err = metadata.Parse(reader, &deserialized)
	if err != nil {
		t.Logf("Raw serialized json: %s", buf.String())
		t.Error(err)
	}

	diff := deep.Equal(testInventory, deserialized)
	if diff != nil {
		t.Error(diff)
	}
}

func TestParseBadInput(t *testing.T) {

	err := metadata.Parse(strings.NewReader("bad json"), &metadata.Inventory{})
	if err == nil {
		t.Fatal("Parser should have thrown an error")
	}
}

func TestInventoryFiles(t *testing.T) {
	v1 := testInventory.Versions["v1"]
	v2 := testInventory.Versions["v2"]
	v3 := testInventory.Versions["v3"]
	cases := map[string][]metadata.File{
		"v1": {
			{
				Version:      &v1,
				Inventory:    &testInventory,
				PhysicalPath: "v1/content/physical/1",
				LogicalPath:  "logical/1",
			},
			{
				Version:      &v1,
				Inventory:    &testInventory,
				PhysicalPath: "v2/content/physical/2",
				LogicalPath:  "logical/2",
			},
		},
		"v2": {
			{
				Version:      &v2,
				Inventory:    &testInventory,
				LogicalPath:  "logical/1",
				PhysicalPath: "v1/content/physical/1",
			},
			{
				Version:      &v2,
				Inventory:    &testInventory,
				LogicalPath:  "logical/3",
				PhysicalPath: "v2/content/physical/3",
			},
		},
		"v3": {
			{
				Version:      &v3,
				Inventory:    &testInventory,
				LogicalPath:  "logical/1",
				PhysicalPath: "v2/content/physical/2",
			},
			{
				Version:      &v3,
				Inventory:    &testInventory,
				LogicalPath:  "logical/2",
				PhysicalPath: "v2/content/physical/3",
			},
			{
				Version:      &v3,
				Inventory:    &testInventory,
				LogicalPath:  "logical/2.copy",
				PhysicalPath: "v2/content/physical/3",
			},
		},
	}

	for v, e := range cases {
		version := v
		expected := e
		vfiles, err := testInventory.Files(version)

		t.Run(version, func(t *testing.T) {
			if err != nil {
				t.Errorf("error while retrieving files %s", err)
			}
			if len(vfiles) != len(expected) {
				t.Errorf("found %d files from %s, but got %d", len(vfiles), version, len(expected))
			}

			for _, file := range expected {
				if !foundFile(file, vfiles) {
					t.Errorf("Did not find file %s (%s) in files from %s", file.LogicalPath, file.PhysicalPath, version)
				}
			}
		})
	}
}

func foundFile(file metadata.File, files []metadata.File) bool {
	for _, f := range files {
		if deep.Equal(file, f) == nil {
			return true
		}
	}
	return false
}

// If there are multiple choices for physical path for a given file in a given version, pick
// the one that most closely matches the desired version.
func TestInventoryFilePhysicalPaths(t *testing.T) {

	const (
		pathv1 = "v1/content/file.bin"
		pathv2 = "v2/content/file.bin"
		pathv3 = "v3/content/file.bin"
	)

	inv := &metadata.Inventory{
		Manifest: metadata.Manifest{
			"a": {}, // Each test case will substitute different values here
		},
		Versions: map[string]metadata.Version{
			"v1": {
				State: metadata.Manifest{
					"a": {"logical/path1"},
				},
			},
			"v2": {
				State: metadata.Manifest{
					"a": {"logical/path1"},
				},
			},
			"v3": {
				State: metadata.Manifest{
					"a": {"logical/path1"},
				},
			},
		},
	}

	cases := []struct {
		name     string
		paths    []string
		expected string
	}{
		{"one choice < v2", []string{pathv1}, pathv1},
		{"v1 v2, pick v2", []string{pathv1, pathv2}, pathv2},
		{"v1 v2, v3, pick v2", []string{pathv1, pathv2, pathv3}, pathv2},
		{"v1, v3, pick v1", []string{pathv1, pathv3}, pathv1},
		{"one choice > v2", []string{pathv3}, pathv3},
	}

	for _, tc := range cases {
		paths := tc.paths
		expected := tc.expected

		t.Run(tc.name, func(t *testing.T) {

			inv.Manifest["a"] = paths

			files, err := inv.Files("v2")
			if err != nil {
				t.Error(err)
			}

			if files[0].PhysicalPath != expected {
				t.Errorf("expected %s but got %s", expected, files[0].PhysicalPath)
			}
		})
	}
}

func TestInventoryFileErrorsBadVersion(t *testing.T) {
	_, err := testInventory.Files("NOOO")
	if err == nil {
		t.Error("Bad version name should have thrown an error")
	}
}

func TestVersionValidity(t *testing.T) {
	cases := map[metadata.VersionID]bool{
		"":        false,
		"v":       false,
		"v0":      false,
		"v0000":   false,
		"v1":      true,
		"v0001":   true,
		"v01d":    false,
		"rhubarb": false,
	}

	for v, e := range cases {
		v := v
		expected := e
		t.Run(string(v), func(t *testing.T) {
			if v.Valid() != expected {
				t.Errorf("%s validity was %t, when expected %t", v, v.Valid(), expected)
			}
		})
	}
}

func TestVersionIncrement(t *testing.T) {
	cases := map[metadata.VersionID]struct {
		version metadata.VersionID
		error   bool
	}{
		"v":     {"", true},
		"v1":    {"v2", false},
		"v9":    {"v10", false},
		"v01":   {"v02", false},
		"v0999": {"v1000", false},
	}

	for before, expected := range cases {
		before := before
		expected := expected
		t.Run(string(before), func(t *testing.T) {
			after, err := before.Increment()
			if (err != nil) != expected.error {
				t.Error("Bad error status")
			}

			if after != expected.version {
				t.Errorf("Increment of %s resulted in %s instead of %s", before, after, expected.version)
			}
		})
	}
}

func TestPutFile(t *testing.T) {
	before := func() *metadata.Inventory {
		return &metadata.Inventory{
			Head: "v2",
			Manifest: metadata.Manifest{
				"a": {"v1/content/a"},
				"b": {"v2/content/b"},
			},
			Versions: map[string]metadata.Version{
				"v1": {
					State: metadata.Manifest{
						"a": {"logical/a"},
					},
				},
				"v2": {
					State: metadata.Manifest{
						"a": {"logical/a"},
						"b": {"logical/b"},
					},
				},
			},
		}
	}

	cases := []struct {
		name         string
		logicalPath  string
		physicalPath string
		digest       metadata.Digest
		expected     metadata.Inventory
	}{
		{
			name:         "putNewFile",
			logicalPath:  "logical/c",
			physicalPath: "v2/content/c",
			digest:       "c",
			expected: metadata.Inventory{
				Head: "v2",
				Manifest: metadata.Manifest{
					"a": {"v1/content/a"},
					"b": {"v2/content/b"},
					"c": {"v2/content/c"},
				},
				Versions: map[string]metadata.Version{
					"v1": {
						State: metadata.Manifest{
							"a": {"logical/a"},
						},
					},
					"v2": {
						State: metadata.Manifest{
							"a": {"logical/a"},
							"b": {"logical/b"},
							"c": {"logical/c"},
						},
					},
				},
			},
		},
		{
			name:         "replaceFile",
			logicalPath:  "logical/b",
			physicalPath: "v2/content/b",
			digest:       "c",
			expected: metadata.Inventory{
				Head: "v2",
				Manifest: metadata.Manifest{
					"a": {"v1/content/a"},
					"c": {"v2/content/b"},
				},
				Versions: map[string]metadata.Version{
					"v1": {
						State: metadata.Manifest{
							"a": {"logical/a"},
						},
					},
					"v2": {
						State: metadata.Manifest{
							"a": {"logical/a"},
							"c": {"logical/b"},
						},
					},
				},
			},
		},
		{
			name:         "hashExists",
			logicalPath:  "logical/c",
			physicalPath: "v2/content/c",
			digest:       "b",
			expected: metadata.Inventory{
				Head: "v2",
				Manifest: metadata.Manifest{
					"a": {"v1/content/a"},
					"b": {"v2/content/b", "v2/content/c"},
				},
				Versions: map[string]metadata.Version{
					"v1": {
						State: metadata.Manifest{
							"a": {"logical/a"},
						},
					},
					"v2": {
						State: metadata.Manifest{
							"a": {"logical/a"},
							"b": {"logical/b", "logical/c"},
						},
					},
				},
			},
		},
		{
			name:         "idempotent",
			logicalPath:  "logical/b",
			physicalPath: "v2/content/b",
			digest:       "b",
			expected:     *before(),
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			inv := before()
			err := inv.PutFile(c.logicalPath, c.physicalPath, c.digest)
			if err != nil {
				t.Fatalf("error invoking PutFile %+v", err)
			}

			mismatches := deep.Equal(&c.expected, inv)
			if len(mismatches) > 0 {
				t.Fatalf("errors in expected content: %s,\n got: %+v", mismatches, inv)
			}
		})
	}
}

func TestPutBadPath(t *testing.T) {
	inv := metadata.NewInventory("id")

	cases := map[string]string{
		"absoluePath":  "/v1/content/foo",
		"wrongVersion": "v2/content/foo",
	}
	for name, path := range cases {
		name := name
		path := path
		t.Run(name, func(t *testing.T) {
			err := inv.PutFile(name, path, "moo")
			if err == nil {
				t.Fatalf("Should have thrown an error")
			}
		})
	}
}

func TestNewInventory(t *testing.T) {
	id := "foo"
	inv := metadata.NewInventory(id)

	cases := []struct {
		name     string
		expected string
		actual   string
	}{
		{"type", metadata.InventoryType, inv.Type},
		{"head", "v1", inv.Head},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			if c.expected != c.actual {
				t.Fatalf("Expected %s, but got %s", c.expected, c.actual)
			}
		})
	}
}
