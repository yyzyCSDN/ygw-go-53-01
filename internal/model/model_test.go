package model

import "testing"

func TestSnapshotCloneIsIndependent(t *testing.T) {
	original := &Snapshot{
		Entity:    "user_1",
		VersionID: "v1",
		Fields:    map[string]FeatureValue{"age": IntValue(30), "city": StringValue("杭州")},
	}
	copy := original.Clone()
	copy.Fields["age"] = IntValue(31)
	if original.Fields["age"].Int != 30 {
		t.Fatalf("clone shares mutable field data: %v", original.Fields["age"])
	}
	if copy.Fields["city"].Str != "杭州" {
		t.Fatalf("clone lost string field: %v", copy.Fields["city"])
	}
}

func TestFeatureValueString(t *testing.T) {
	cases := []struct {
		value FeatureValue
		want  string
	}{
		{StringValue("abc"), "abc"},
		{IntValue(42), "42"},
		{FloatValue(3.5), "3.5"},
		{BoolValue(true), "true"},
	}
	for _, c := range cases {
		if got := c.value.String(); got != c.want {
			t.Errorf("String() = %q, want %q", got, c.want)
		}
	}
}

func TestFieldNamesSorted(t *testing.T) {
	snap := EmptySnapshot("e", "v")
	snap.Fields["z"] = IntValue(1)
	snap.Fields["a"] = IntValue(2)
	names := snap.FieldNames()
	if names[0] != "a" || names[1] != "z" {
		t.Fatalf("field names not sorted: %v", names)
	}
}

func TestTaskCompleted(t *testing.T) {
	task := &Task{State: TaskDone, Total: 10, Processed: 10}
	if !task.Completed() {
		t.Fatal("done task with all items processed should be completed")
	}
	task.Processed = 9
	if task.Completed() {
		t.Fatal("task with unprocessed items should not be completed")
	}
}
