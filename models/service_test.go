package models

import "testing"

func TestMirakurunID(t *testing.T) {
	cases := []struct {
		name      string
		networkID int64
		serviceID int64
		want      int64
	}{
		{"terrestrial TV Tokyo", 32742, 1072, 3274201072},
		{"BS NHK1", 4, 101, 400101},
		{"CS channel", 7, 300, 700300},
		{"zero network", 0, 1234, 1234},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := MirakurunID(c.networkID, c.serviceID)
			if got != c.want {
				t.Errorf("MirakurunID(%d, %d) = %d, want %d", c.networkID, c.serviceID, got, c.want)
			}
		})
	}
}

func TestServiceMapKeyedByMirakurunID(t *testing.T) {
	sm := NewServiceMap()

	// Two services sharing serviceID across different networks
	a := &Service{ID: MirakurunID(32742, 1072), ServiceID: 1072, NetworkID: 32742, Name: "テレ東"}
	b := &Service{ID: MirakurunID(32736, 1072), ServiceID: 1072, NetworkID: 32736, Name: "BS something"}

	sm.Add(a)
	sm.Add(b)

	gotA, ok := sm.Get(a.ID)
	if !ok || gotA.Name != "テレ東" {
		t.Errorf("Get(%d) = %+v, ok=%v; want テレ東", a.ID, gotA, ok)
	}
	gotB, ok := sm.Get(b.ID)
	if !ok || gotB.Name != "BS something" {
		t.Errorf("Get(%d) = %+v, ok=%v; want BS something", b.ID, gotB, ok)
	}

	all := sm.GetByServiceID(1072)
	if len(all) != 2 {
		t.Fatalf("GetByServiceID(1072) len=%d, want 2", len(all))
	}
}

func TestGetByServiceIDIsDeterministic(t *testing.T) {
	// Register services in a semi-random order and confirm returned order is
	// always service.ID ascending regardless of insertion order.
	networkIDs := []int64{32742, 32736, 4, 32740, 7, 100, 32738}
	serviceID := int64(1072)
	sm := NewServiceMap()
	for _, n := range networkIDs {
		sm.Add(&Service{ID: MirakurunID(n, serviceID), ServiceID: serviceID, NetworkID: n})
	}
	want := []int64{
		MirakurunID(4, serviceID),
		MirakurunID(7, serviceID),
		MirakurunID(100, serviceID),
		MirakurunID(32736, serviceID),
		MirakurunID(32738, serviceID),
		MirakurunID(32740, serviceID),
		MirakurunID(32742, serviceID),
	}
	// sanity: want must be sorted ascending
	for i := 1; i < len(want); i++ {
		if want[i-1] >= want[i] {
			t.Fatalf("want slice not sorted: %v", want)
		}
	}

	for i := 0; i < 5; i++ {
		got := sm.GetByServiceID(serviceID)
		if len(got) != len(want) {
			t.Fatalf("GetByServiceID len=%d want=%d", len(got), len(want))
		}
		for j, s := range got {
			if s.ID != want[j] {
				t.Errorf("iter %d pos %d: got ID=%d want %d", i, j, s.ID, want[j])
			}
		}
	}
}

func TestRemoveByServiceID(t *testing.T) {
	sm := NewServiceMap()
	sm.Add(&Service{ID: MirakurunID(32742, 1072), ServiceID: 1072, NetworkID: 32742, Name: "テレ東"})
	sm.Add(&Service{ID: MirakurunID(32736, 1072), ServiceID: 1072, NetworkID: 32736, Name: "BS dup"})
	sm.Add(&Service{ID: MirakurunID(32736, 101), ServiceID: 101, NetworkID: 32736, Name: "BS NHK1"})

	n := sm.RemoveByServiceID(1072)
	if n != 2 {
		t.Errorf("RemoveByServiceID(1072) removed %d, want 2", n)
	}
	if got := sm.GetByServiceID(1072); len(got) != 0 {
		t.Errorf("after RemoveByServiceID(1072): len=%d want 0", len(got))
	}
	if got := sm.GetByServiceID(101); len(got) != 1 {
		t.Errorf("unrelated service removed: len=%d want 1", len(got))
	}
}

func TestServiceMapRemoveByMirakurunID(t *testing.T) {
	sm := NewServiceMap()
	a := &Service{ID: MirakurunID(32742, 1072), ServiceID: 1072, NetworkID: 32742, Name: "テレ東"}
	b := &Service{ID: MirakurunID(32736, 1072), ServiceID: 1072, NetworkID: 32736, Name: "BS"}
	sm.Add(a)
	sm.Add(b)

	sm.Remove(a.ID)
	if _, ok := sm.Get(a.ID); ok {
		t.Errorf("Get(%d) after Remove: still present", a.ID)
	}
	if _, ok := sm.Get(b.ID); !ok {
		t.Errorf("Get(%d) after Remove of a: b disappeared", b.ID)
	}
}
