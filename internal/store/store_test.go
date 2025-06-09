package store

import (
	"os"
	"testing"
	"time"
)

func TestStoreCRUD(t *testing.T) {
	tmp, err := os.CreateTemp("", "storetest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	s, err := NewStore(tmp.Name())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	info := ContainerInfo{ID: "1", Name: "n", Image: "img", PID: 123, State: "Running", StartedAt: time.Now(), RestartMax: 0, Ports: "8080:80"}
	if err := s.SaveContainer(info); err != nil {
		t.Fatalf("save: %v", err)
	}
	list, err := s.ListContainers()
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v len=%d", err, len(list))
	}
	if _, err := s.GetContainer("1"); err != nil {
		t.Fatalf("get: %v", err)
	}
	if err := s.UpdateContainerState("1", "Stopped"); err != nil {
		t.Fatalf("update: %v", err)
	}
	list, err = s.ListContainers()
	if err != nil || list[0].State != "Stopped" {
		t.Fatalf("state not updated")
	}
	if err := s.DeleteContainer("1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	list, err = s.ListContainers()
	if err != nil || len(list) != 0 {
		t.Fatalf("delete check failed")
	}

	img := ImageInfo{Name: "busybox", Path: "/tmp/busybox", CreatedAt: time.Now()}
	if err := s.SaveImage(img); err != nil {
		t.Fatalf("save image: %v", err)
	}
	if _, err := s.GetImage("busybox"); err != nil {
		t.Fatalf("get image: %v", err)
	}
	imgs, err := s.ListImages()
	if err != nil || len(imgs) != 1 {
		t.Fatalf("list images failed")
	}
}
