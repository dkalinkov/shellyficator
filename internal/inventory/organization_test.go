package inventory

import (
	"reflect"
	"strings"
	"testing"

	"shelly-manager/internal/shelly"
)

func TestOrganizationReusesLabelsAndPreservesDeviceData(t *testing.T) {
	store := testStore(t)
	reading := shelly.Reading{Identity: shelly.Identity{ID: "first", MAC: "AABBCCDDEEFF", Gen: 4}, Address: "http://192.168.1.2", Status: map[string]any{"switch:0": map[string]any{"output": false}}, Config: map[string]any{"switch:0": map[string]any{"in_mode": "momentary"}}}
	first, err := store.SaveReading(reading, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Metadata(first.Id, "Bedroom", "Lighting, ground floor", "Keep this note"); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkBaseline(first.Id); err != nil {
		t.Fatal(err)
	}
	first, _ = store.Device(first.Id)
	baseline, config, status := first.GetString("baseline"), first.GetString("config"), first.GetString("status")
	if err := store.SaveChannelNote(first.Id, "switch:0", "Ceiling lights"); err != nil {
		t.Fatal(err)
	}
	reading.Identity.ID, reading.Identity.MAC = "second", "AABBCCDDEEAA"
	second, err := store.SaveReading(reading, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Metadata(second.Id, "Kitchen", "Heating", "Another note"); err != nil {
		t.Fatal(err)
	}
	if err := store.Organization(first.Id, "  kitchen ", " lighting, LIGHTING, new label, \n ground   floor ,, "); err != nil {
		t.Fatal(err)
	}
	first, _ = store.Device(first.Id)
	if first.GetString("room") != "Kitchen" || first.GetString("tags") != "Lighting, new label, ground floor" {
		t.Fatalf("existing labels were duplicated or multiword labels lost: room=%q tags=%q", first.GetString("room"), first.GetString("tags"))
	}
	if first.GetString("notes") != "Keep this note" || first.GetString("baseline") != baseline || first.GetString("config") != config || first.GetString("status") != status || store.ChannelNotes(first.Id)["switch:0"] != "Ceiling lights" {
		t.Fatal("organization edit changed other device data")
	}
	rooms, tags, err := store.OrganizationOptions()
	if err != nil || !reflect.DeepEqual(rooms, []string{"Kitchen"}) || !reflect.DeepEqual(tags, []string{"ground floor", "Heating", "Lighting", "new label"}) {
		t.Fatalf("unexpected catalog: rooms=%v tags=%v err=%v", rooms, tags, err)
	}
	if err := store.Organization(first.Id, "", ""); err != nil {
		t.Fatal(err)
	}
	first, _ = store.Device(first.Id)
	if first.GetString("room") != "" || first.GetString("tags") != "" || first.GetString("notes") != "Keep this note" {
		t.Fatal("clearing assignments discarded the note or did not clear labels")
	}
}

func TestOrganizationRejectsOversizedLabelsAtomically(t *testing.T) {
	store := testStore(t)
	device, err := store.SaveReading(shelly.Reading{Identity: shelly.Identity{ID: "test", MAC: "AABBCCDDEEFF", Gen: 4}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Metadata(device.Id, "Bedroom", "lighting", "Saved note"); err != nil {
		t.Fatal(err)
	}
	for _, input := range []struct{ room, tags string }{{strings.Repeat("я", 101), "new tag"}, {"Kitchen", strings.Repeat("x", 501)}} {
		if err := store.Organization(device.Id, input.room, input.tags); err == nil {
			t.Fatal("oversized organization data was accepted")
		}
		saved, _ := store.Device(device.Id)
		if saved.GetString("room") != "Bedroom" || saved.GetString("tags") != "lighting" || saved.GetString("notes") != "Saved note" {
			t.Fatal("failed edit partially changed saved metadata")
		}
	}
}
