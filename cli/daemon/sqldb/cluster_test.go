package sqldb

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"os/exec"
	"testing"

	qt "github.com/frankban/quicktest"

	"encr.dev/cli/daemon/internal/runlog"
)

func TestClusterManager_StartDelete(t *testing.T) {
	if testing.Short() {
		t.Log("skipping cluster test when running in short mode")
		t.SkipNow()
	}
	c := qt.New(t)
	cm := NewClusterManager()
	ctx := context.Background()
	clusterID := genClusterID(c)
	cl := cm.Init(ctx, &InitParams{ClusterID: clusterID})
	err := cl.Start(runlog.OS())
	c.Assert(err, qt.IsNil)
	c.Assert(cl, qt.Not(qt.IsNil))

	cname := containerName(clusterID)
	err = exec.Command("docker", "container", "inspect", cname).Run()
	c.Assert(err, qt.IsNil)

	err = cm.Delete(ctx, clusterID)
	c.Assert(err, qt.IsNil)
	out, err := exec.Command("docker", "container", "inspect", cname).CombinedOutput()
	c.Assert(err, qt.Not(qt.IsNil))
	c.Assert(string(out), qt.Contains, "No such container")
}

func TestClusterManager_Get(t *testing.T) {
	if testing.Short() {
		t.Log("skipping cluster test when running in short mode")
		t.SkipNow()
	}

	c := qt.New(t)
	cm := NewClusterManager()
	cl := testCluster(c, cm)
	cluster, ok := cm.Get(cl.ID)
	c.Assert(ok, qt.IsTrue)
	c.Assert(cluster, qt.Equals, cl)
	c.Assert(cluster, qt.Not(qt.IsNil))
	c.Assert(cluster.ID, qt.Equals, cl.ID)
}

func testCluster(c *qt.C, cm *ClusterManager) *Cluster {
	ctx := context.Background()
	clusterID := genClusterID(c)
	cl := cm.Init(ctx, &InitParams{ClusterID: clusterID})
	err := cl.Start(runlog.OS())
	c.Assert(err, qt.IsNil)
	c.Assert(cl, qt.Not(qt.IsNil))
	c.Cleanup(func() {
		err := cm.Delete(context.Background(), clusterID)
		c.Assert(err, qt.IsNil)
	})
	return cl
}

var encoding = base32.NewEncoding("23456789abcdefghikmnopqrstuvwxyz").WithPadding(base32.NoPadding)

func genClusterID(c *qt.C) string {
	var data [3]byte
	_, err := rand.Read(data[:])
	c.Assert(err, qt.IsNil)
	return "sqldb-internal-test-" + encoding.EncodeToString(data[:])
}
