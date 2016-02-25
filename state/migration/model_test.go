// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package migration

import (
	"fmt"
	"time"

	"github.com/juju/errors"
	"github.com/juju/names"
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"
	"gopkg.in/yaml.v2"

	"github.com/juju/juju/testing"
	"github.com/juju/juju/version"
)

type ModelSerializationSuite struct {
	testing.BaseSuite
}

var _ = gc.Suite(&ModelSerializationSuite{})

func (*ModelSerializationSuite) TestNil(c *gc.C) {
	_, err := importModel(nil)
	c.Check(err, gc.ErrorMatches, "version: expected int, got nothing")
}

func (*ModelSerializationSuite) TestMissingVersion(c *gc.C) {
	_, err := importModel(map[string]interface{}{})
	c.Check(err, gc.ErrorMatches, "version: expected int, got nothing")
}

func (*ModelSerializationSuite) TestNonIntVersion(c *gc.C) {
	_, err := importModel(map[string]interface{}{
		"version": "hello",
	})
	c.Check(err.Error(), gc.Equals, `version: expected int, got string("hello")`)
}

func (*ModelSerializationSuite) TestUnknownVersion(c *gc.C) {
	_, err := importModel(map[string]interface{}{
		"version": 42,
	})
	c.Check(err.Error(), gc.Equals, `version 42 not valid`)
}

func (*ModelSerializationSuite) modelMap() map[string]interface{} {
	latestTools := version.MustParse("2.0.1")
	configMap := map[string]interface{}{
		"name": "awesome",
		"uuid": "some-uuid",
	}
	return map[string]interface{}{
		"version":      1,
		"owner":        "magic",
		"config":       configMap,
		"latest-tools": latestTools.String(),
		"users": map[string]interface{}{
			"version": 1,
			"users": []interface{}{
				map[string]interface{}{
					"name":         "admin@local",
					"created-by":   "admin@local",
					"date-created": time.Date(2015, 10, 9, 12, 34, 56, 0, time.UTC),
				},
			},
		},
		"machines": map[string]interface{}{
			"version": 1,
			"machines": []interface{}{
				minimalMachineMap("0"),
			},
		},
		"services": map[string]interface{}{
			"version": 1,
			"services": []interface{}{
				minimalServiceMap(),
			},
		},
		"relations": map[string]interface{}{
			"version":   1,
			"relations": []interface{}{},
		},
	}
}

func (s *ModelSerializationSuite) TestParsingYAML(c *gc.C) {
	initial := s.modelMap()
	bytes, err := yaml.Marshal(initial)
	c.Assert(err, jc.ErrorIsNil)

	model, err := DeserializeModel(bytes)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(model.Owner(), gc.Equals, names.NewUserTag("magic"))
	c.Assert(model.Tag().Id(), gc.Equals, "some-uuid")
	c.Assert(model.Config(), jc.DeepEquals, initial["config"])
	c.Assert(model.LatestToolsVersion(), gc.Equals, version.MustParse("2.0.1"))
	users := model.Users()
	c.Assert(users, gc.HasLen, 1)
	c.Assert(users[0].Name(), gc.Equals, names.NewUserTag("admin@local"))
	machines := model.Machines()
	c.Assert(machines, gc.HasLen, 1)
	c.Assert(machines[0].Id(), gc.Equals, "0")
	services := model.Services()
	c.Assert(services, gc.HasLen, 1)
	c.Assert(services[0].Name(), gc.Equals, "ubuntu")
}

func (*ModelSerializationSuite) TestParsingOptionals(c *gc.C) {
	configMap := map[string]interface{}{
		"name": "awesome",
		"uuid": "some-uuid",
	}
	model, err := importModel(map[string]interface{}{
		"version": 1,
		"owner":   "magic",
		"config":  configMap,
		"users": map[string]interface{}{
			"version": 1,
			"users":   []interface{}{},
		},
		"machines": map[string]interface{}{
			"version":  1,
			"machines": []interface{}{},
		},
		"services": map[string]interface{}{
			"version":  1,
			"services": []interface{}{},
		},
		"relations": map[string]interface{}{
			"version":   1,
			"relations": []interface{}{},
		},
	})
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(model.LatestToolsVersion(), gc.Equals, version.Zero)
}

func (s *ModelSerializationSuite) TestAnnotations(c *gc.C) {
	initial := NewModel(ModelArgs{Owner: names.NewUserTag("owner")})
	annotations := map[string]string{
		"string":  "value",
		"another": "one",
	}
	initial.SetAnnotations(annotations)

	bytes, err := yaml.Marshal(initial)
	c.Assert(err, jc.ErrorIsNil)

	model, err := DeserializeModel(bytes)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(model.Annotations(), jc.DeepEquals, annotations)
}

func (*ModelSerializationSuite) TestModelValidation(c *gc.C) {
	model := NewModel(ModelArgs{})
	err := model.Validate()
	c.Assert(err, gc.ErrorMatches, "missing model owner not valid")
	c.Assert(err, jc.Satisfies, errors.IsNotValid)
}

func (*ModelSerializationSuite) TestModelValidationChecksMachines(c *gc.C) {
	model := NewModel(ModelArgs{Owner: names.NewUserTag("owner")})
	model.AddMachine(MachineArgs{})
	err := model.Validate()
	c.Assert(err, gc.ErrorMatches, "machine missing id not valid")
	c.Assert(err, jc.Satisfies, errors.IsNotValid)
}

func (s *ModelSerializationSuite) addMachineToModel(model Model, id string) Machine {
	machine := model.AddMachine(MachineArgs{Id: names.NewMachineTag(id)})
	machine.SetInstance(CloudInstanceArgs{InstanceId: "magic"})
	machine.SetTools(minimalAgentToolsArgs())
	machine.SetStatus(minimalStatusArgs())
	return machine
}

func (s *ModelSerializationSuite) TestModelValidationChecksMachinesGood(c *gc.C) {
	model := NewModel(ModelArgs{Owner: names.NewUserTag("owner")})
	s.addMachineToModel(model, "0")
	err := model.Validate()
	c.Assert(err, jc.ErrorIsNil)
}

func (s *ModelSerializationSuite) TestModelValidationChecksOpenPortsUnits(c *gc.C) {
	model := NewModel(ModelArgs{Owner: names.NewUserTag("owner")})
	machine := s.addMachineToModel(model, "0")
	machine.AddNetworkPorts(NetworkPortsArgs{
		OpenPorts: []PortRangeArgs{
			{
				UnitName: "missing/0",
				FromPort: 8080,
				ToPort:   8080,
				Protocol: "tcp",
			},
		},
	})
	err := model.Validate()
	c.Assert(err.Error(), gc.Equals, "unknown unit names in open ports: [missing/0]")
}

func (*ModelSerializationSuite) TestModelValidationChecksServices(c *gc.C) {
	model := NewModel(ModelArgs{Owner: names.NewUserTag("owner")})
	model.AddService(ServiceArgs{})
	err := model.Validate()
	c.Assert(err, gc.ErrorMatches, "service missing name not valid")
	c.Assert(err, jc.Satisfies, errors.IsNotValid)
}

func (s *ModelSerializationSuite) addServiceToModel(model Model, name string, numUnits int) Service {
	service := model.AddService(ServiceArgs{
		Tag:                names.NewServiceTag(name),
		Settings:           map[string]interface{}{},
		LeadershipSettings: map[string]interface{}{},
	})
	service.SetStatus(minimalStatusArgs())
	for i := 0; i < numUnits; i++ {
		// The index i is used as both the machine id and the unit id.
		// A happy coincidence.
		machine := s.addMachineToModel(model, fmt.Sprint(i))
		unit := service.AddUnit(UnitArgs{
			Tag:     names.NewUnitTag(fmt.Sprintf("%s/%d", name, i)),
			Machine: machine.Tag(),
		})
		unit.SetTools(minimalAgentToolsArgs())
		unit.SetAgentStatus(minimalStatusArgs())
		unit.SetWorkloadStatus(minimalStatusArgs())
	}

	return service
}

func (s *ModelSerializationSuite) wordpressModel() (Model, Endpoint, Endpoint) {
	model := NewModel(ModelArgs{
		Owner: names.NewUserTag("owner"),
		Config: map[string]interface{}{
			"uuid": "some-uuid",
		}})
	s.addServiceToModel(model, "wordpress", 2)
	s.addServiceToModel(model, "mysql", 1)

	// Add a relation between wordpress and mysql.
	rel := model.AddRelation(RelationArgs{
		Id:  42,
		Key: "special key",
	})
	wordpressEndpoint := rel.AddEndpoint(EndpointArgs{
		ServiceName: "wordpress",
		Name:        "db",
		// Ignoring other aspects of endpoints.
	})
	mysqlEndpoint := rel.AddEndpoint(EndpointArgs{
		ServiceName: "mysql",
		Name:        "mysql",
		// Ignoring other aspects of endpoints.
	})
	return model, wordpressEndpoint, mysqlEndpoint
}

func (s *ModelSerializationSuite) wordpressModelWithSettings() Model {
	model, wordpressEndpoint, mysqlEndpoint := s.wordpressModel()

	wordpressEndpoint.SetUnitSettings("wordpress/0", map[string]interface{}{
		"key": "value",
	})
	wordpressEndpoint.SetUnitSettings("wordpress/1", map[string]interface{}{
		"key": "value",
	})
	mysqlEndpoint.SetUnitSettings("mysql/0", map[string]interface{}{
		"key": "value",
	})
	return model
}

func (s *ModelSerializationSuite) TestModelValidationChecksRelationsMissingSettings(c *gc.C) {
	model, _, _ := s.wordpressModel()
	err := model.Validate()
	c.Assert(err, gc.ErrorMatches, "missing relation settings for units \\[wordpress/0 wordpress/1\\] in relation 42")
}

func (s *ModelSerializationSuite) TestModelValidationChecksRelationsMissingSettings2(c *gc.C) {
	model, wordpressEndpoint, _ := s.wordpressModel()

	wordpressEndpoint.SetUnitSettings("wordpress/0", map[string]interface{}{
		"key": "value",
	})
	wordpressEndpoint.SetUnitSettings("wordpress/1", map[string]interface{}{
		"key": "value",
	})
	err := model.Validate()
	c.Assert(err, gc.ErrorMatches, "missing relation settings for units \\[mysql/0\\] in relation 42")
}

func (s *ModelSerializationSuite) TestModelValidationChecksRelations(c *gc.C) {
	model := s.wordpressModelWithSettings()
	err := model.Validate()
	c.Assert(err, jc.ErrorIsNil)
}

func (s *ModelSerializationSuite) TestModelSerializationWithRelations(c *gc.C) {
	initial := s.wordpressModelWithSettings()
	bytes, err := yaml.Marshal(initial)
	c.Assert(err, jc.ErrorIsNil)
	model, err := DeserializeModel(bytes)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(model, jc.DeepEquals, initial)
}