// Package images reproduces github.com/docker/docker@v28.5.1+incompatible daemon/images/image_list.go:111 (ImageService.Images):
// filter a map[image.ID]*image.Image, build summaries, sort.Sort(sort.Reverse(byCreated)).
// Created is Unix seconds, so images built in the same second tie. image.ID is a string.
package images

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"

	"fullcoverage/cases/variant"
)

type Image struct {
	Created *time.Time
	Labels  map[string]string
}

type Summary struct {
	ID      string
	Created int64
}

type byCreated []*Summary

func (r byCreated) Len() int           { return len(r) }
func (r byCreated) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r byCreated) Less(i, j int) bool { return r[i].Created < r[j].Created }

// byCreatedThenID is S4's total Less.
type byCreatedThenID []*Summary

func (r byCreatedThenID) Len() int      { return len(r) }
func (r byCreatedThenID) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r byCreatedThenID) Less(i, j int) bool {
	return cmp.Or(cmp.Compare(r[i].Created, r[j].Created), strings.Compare(r[i].ID, r[j].ID)) < 0
}

// Original is the shipped shape.
func Original(selected map[string]*Image, before time.Time) []*Summary {
	summaries := make([]*Summary, 0, len(selected))
	for id, img := range selected {
		if !before.IsZero() && (img.Created == nil || !img.Created.Before(before)) {
			continue
		}
		if img.Labels["skip"] != "" {
			continue
		}
		summaries = append(summaries, &Summary{ID: id, Created: img.Created.Unix()})
	}
	sort.Sort(sort.Reverse(byCreated(summaries)))
	return summaries
}

// S1 ranges the sorted IDs and makes the sort stable.
func S1(selected map[string]*Image, before time.Time) []*Summary {
	summaries := make([]*Summary, 0, len(selected))
	for _, id := range slices.Sorted(maps.Keys(selected)) {
		img := selected[id]
		if !before.IsZero() && (img.Created == nil || !img.Created.Before(before)) {
			continue
		}
		if img.Labels["skip"] != "" {
			continue
		}
		summaries = append(summaries, &Summary{ID: id, Created: img.Created.Unix()})
	}
	sort.Stable(sort.Reverse(byCreated(summaries)))
	return summaries
}

// S1Unstable ranges the sorted IDs but keeps the unstable sort.Sort.
func S1Unstable(selected map[string]*Image, before time.Time) []*Summary {
	summaries := make([]*Summary, 0, len(selected))
	for _, id := range slices.Sorted(maps.Keys(selected)) {
		img := selected[id]
		if !before.IsZero() && (img.Created == nil || !img.Created.Before(before)) {
			continue
		}
		if img.Labels["skip"] != "" {
			continue
		}
		summaries = append(summaries, &Summary{ID: id, Created: img.Created.Unix()})
	}
	sort.Sort(sort.Reverse(byCreated(summaries)))
	return summaries
}

// S4 keeps the map loop and makes Less total.
func S4(selected map[string]*Image, before time.Time) []*Summary {
	summaries := make([]*Summary, 0, len(selected))
	for id, img := range selected {
		if !before.IsZero() && (img.Created == nil || !img.Created.Before(before)) {
			continue
		}
		if img.Labels["skip"] != "" {
			continue
		}
		summaries = append(summaries, &Summary{ID: id, Created: img.Created.Unix()})
	}
	sort.Sort(sort.Reverse(byCreatedThenID(summaries)))
	return summaries
}

func fixture() map[string]*Image {
	m := map[string]*Image{}
	base := time.Unix(1_700_000_000, 0)
	for i := range 12 {
		created := base.Add(time.Duration(i/4) * time.Second).Add(time.Duration(i) * time.Millisecond)
		m[fmt.Sprintf("sha256:%02d", i)] = &Image{Created: &created}
	}
	return m
}

func render(f func(map[string]*Image, time.Time) []*Summary) func() string {
	return func() string {
		var b strings.Builder
		for _, s := range f(fixture(), time.Time{}) {
			fmt.Fprintf(&b, "%s@%d ", s.ID, s.Created)
		}
		return b.String()
	}
}

var Variants = []variant.Variant{
	{Name: "Original", Run: render(Original)},
	{Name: "S1", Run: render(S1)},
	{Name: "S1Unstable", Run: render(S1Unstable)},
	{Name: "S4", Run: render(S4)},
}
