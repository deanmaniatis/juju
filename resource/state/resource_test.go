// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state_test

import (
	"time"

	"github.com/juju/errors"
	"github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"
	charmresource "gopkg.in/juju/charm.v6-unstable/resource"

	"github.com/juju/juju/resource"
	"github.com/juju/juju/resource/state"
)

var _ = gc.Suite(&ResourceSuite{})

type ResourceSuite struct {
	testing.IsolationSuite

	stub    *testing.Stub
	raw     *stubRawState
	persist *stubPersistence
}

func (s *ResourceSuite) SetUpTest(c *gc.C) {
	s.IsolationSuite.SetUpTest(c)

	s.stub = &testing.Stub{}
	s.raw = &stubRawState{stub: s.stub}
	s.persist = &stubPersistence{stub: s.stub}
	s.raw.ReturnPersistence = s.persist
}

func (s *ResourceSuite) TestListResourcesOkay(c *gc.C) {
	expected := newUploadResources(c, "spam", "eggs")
	s.persist.ReturnListResources = expected
	st := state.NewState(s.raw)
	s.stub.ResetCalls()

	resources, err := st.ListResources("a-service")
	c.Assert(err, jc.ErrorIsNil)

	c.Check(resources, jc.DeepEquals, expected)
	s.stub.CheckCallNames(c, "ListResources")
	s.stub.CheckCall(c, 0, "ListResources", "a-service")
}

func (s *ResourceSuite) TestListResourcesEmpty(c *gc.C) {
	st := state.NewState(s.raw)
	s.stub.ResetCalls()

	resources, err := st.ListResources("a-service")
	c.Assert(err, jc.ErrorIsNil)

	c.Check(resources, gc.HasLen, 0)
	s.stub.CheckCallNames(c, "ListResources")
}

func (s *ResourceSuite) TestListResourcesError(c *gc.C) {
	expected := newUploadResources(c, "spam", "eggs")
	s.persist.ReturnListResources = expected
	st := state.NewState(s.raw)
	s.stub.ResetCalls()
	failure := errors.New("<failure>")
	s.stub.SetErrors(failure)

	_, err := st.ListResources("a-service")

	c.Check(errors.Cause(err), gc.Equals, failure)
	s.stub.CheckCallNames(c, "ListResources")
}

func newUploadResources(c *gc.C, names ...string) []resource.Resource {
	var resources []resource.Resource
	for _, name := range names {
		fp, err := charmresource.GenerateFingerprint([]byte(name))
		c.Assert(err, jc.ErrorIsNil)

		res := resource.Resource{
			Resource: charmresource.Resource{
				Meta: charmresource.Meta{
					Name: name,
					Type: charmresource.TypeFile,
					Path: name + ".tgz",
				},
				Origin:      charmresource.OriginUpload,
				Revision:    0,
				Fingerprint: fp,
			},
			Username:  "a-user",
			Timestamp: time.Now(),
		}
		err = res.Validate()
		c.Assert(err, jc.ErrorIsNil)

		resources = append(resources, res)
	}
	return resources
}