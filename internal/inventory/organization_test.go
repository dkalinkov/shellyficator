package inventory

import (
	"reflect"
	"strings"
	"testing"

	"shellyficator/internal/domain"
)

func TestOrganizationReusesLabelsAndPreservesDeviceData(t *testing.T) {
	store := testStore(t)
	reading := domain.Reading{Identity: domain.Identity{ID: "first", MAC: "AABBCCDDEEFF", Gen: 4}, Address: "http://192.168.1.2", Status: map[string]any{"switch:0": map[string]any{"output": false}}, Config: map[string]any{"switch:0": map[string]any{"in_mode": "momentary"}}}
	first, err := store.ApproveReading(reading)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Metadata(first.ID, "Bedroom", "Lighting, ground floor", "Keep this note"); err != nil {
		t.Fatal(err)
	}
	first, _ = store.Device(first.ID)
	baseline, config, status := first.BaselineID, first.Config, first.Status
	if err := store.SaveChannelNote(first.ID, "switch:0", "Ceiling lights"); err != nil {
		t.Fatal(err)
	}
	reading.Identity.ID, reading.Identity.MAC = "second", "AABBCCDDEEAA"
	second, err := store.SaveReading(reading, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Metadata(second.ID, "Kitchen", "Heating", "Another note"); err != nil {
		t.Fatal(err)
	}
	if err := store.Organization(first.ID, "  kitchen ", " lighting, LIGHTING, new label, \n ground   floor ,, "); err != nil {
		t.Fatal(err)
	}
	first, _ = store.Device(first.ID)
	if first.Room != "Kitchen" || strings.Join(first.Tags, ", ") != "Lighting, new label, ground floor" {
		t.Fatalf("existing labels were duplicated or multiword labels lost: room=%q tags=%q", first.Room, strings.Join(first.Tags, ", "))
	}
	if first.Notes != "Keep this note" || first.BaselineID != baseline || !reflect.DeepEqual(first.Config, config) || !reflect.DeepEqual(first.Status, status) || testChannelNotes(t, store, first.ID)["switch:0"] != "Ceiling lights" {
		t.Fatal("organization edit changed other device data")
	}
	options, err := store.OrganizationOptions()
	if err != nil || !reflect.DeepEqual(options.Rooms, []string{"Kitchen"}) || !reflect.DeepEqual(options.Tags, []string{"ground floor", "Heating", "Lighting", "new label"}) {
		t.Fatalf("unexpected catalog: rooms=%v tags=%v err=%v", options.Rooms, options.Tags, err)
	}
	if err := store.Organization(first.ID, "", ""); err != nil {
		t.Fatal(err)
	}
	first, _ = store.Device(first.ID)
	if first.Room != "" || strings.Join(first.Tags, ", ") != "" || first.Notes != "Keep this note" {
		t.Fatal("clearing assignments discarded the note or did not clear labels")
	}
}

func TestOrganizationRejectsOversizedLabelsAtomically(t *testing.T) {
	store := testStore(t)
	device, err := store.SaveReading(domain.Reading{Identity: domain.Identity{ID: "test", MAC: "AABBCCDDEEFF", Gen: 4}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Metadata(device.ID, "Bedroom", "lighting", "Saved note"); err != nil {
		t.Fatal(err)
	}
	for _, input := range []struct{ room, tags string }{{strings.Repeat("я", 101), "new tag"}, {"Kitchen", strings.Repeat("x", 501)}} {
		if err := store.Organization(device.ID, input.room, input.tags); err == nil {
			t.Fatal("oversized organization data was accepted")
		}
		saved, _ := store.Device(device.ID)
		if saved.Room != "Bedroom" || strings.Join(saved.Tags, ", ") != "lighting" || saved.Notes != "Saved note" {
			t.Fatal("failed edit partially changed saved metadata")
		}
	}
}
