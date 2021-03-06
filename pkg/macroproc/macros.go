package macroproc

import (
	"encoding/base64"
	"encoding/json"

	"fmt"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

// Phases are only used a logical grouping,
// mostly there is no need as modifiers are
// sorted by depth, and we can apply other
// sub-sorting methods if needed, but for
// the time being phases are convinient to
// keep. If it becomes tedious to maintain,
// they can be removed.

const (
	MacrosEvalPhaseA = iota
	MacrosEvalPhaseB
	MacrosEvalPhaseC
	MacrosEvalPhaseD
	MacrosEvalPhaseE
	MacrosEvalPhases
)

var macrosEvalPhases = [MacrosEvalPhases]MacrosEvalPhase{
	MacrosEvalPhaseA,
	MacrosEvalPhaseB,
	MacrosEvalPhaseC,
	MacrosEvalPhaseD,
	MacrosEvalPhaseE,
}

var (
	// Phase A – branching

	MacroBooleanIf = &Macro{
		ReturnType: Null,
		EvalPhase:  MacrosEvalPhaseA,
		VerbName:   "If",
	}

	// Phase B – lookups

	MacroBooleanLookup = &Macro{
		ReturnType: Boolean,
		EvalPhase:  MacrosEvalPhaseB,
		VerbName:   "Lookup",
	}
	MacroStringLookup = &Macro{
		ReturnType: String,
		EvalPhase:  MacrosEvalPhaseB,
		VerbName:   "Lookup",
	}
	MacroNumberLookup = &Macro{
		ReturnType: Number,
		EvalPhase:  MacrosEvalPhaseB,
		VerbName:   "Lookup",
	}
	MacroArrayLookup = &Macro{
		ReturnType: Array,
		EvalPhase:  MacrosEvalPhaseB,
		VerbName:   "Lookup",
	}
	MacroObjectLookup = &Macro{
		ReturnType: Object,
		EvalPhase:  MacrosEvalPhaseB,
		VerbName:   "Lookup",
	}

	// Phase C – importers

	LoadObjectJSON = &Macro{
		ReturnType: Object,
		EvalPhase:  MacrosEvalPhaseC,
		VerbName:   "LoadJSON",
	}
	LoadArrayJSON = &Macro{
		ReturnType: Array,
		EvalPhase:  MacrosEvalPhaseC,
		VerbName:   "LoadJSON",
	}

	// Phase D – string functions

	MacroStringJoin = &Macro{
		ReturnType: String,
		EvalPhase:  MacrosEvalPhaseD,
		VerbName:   "Join",
	}
	MacroStringAsJSON = &Macro{
		ReturnType: String,
		EvalPhase:  MacrosEvalPhaseD,
		VerbName:   "AsJSON",
	}
	MacroStringAsYAML = &Macro{
		ReturnType: String,
		EvalPhase:  MacrosEvalPhaseD,
		VerbName:   "AsYAML",
	}
	MacroStringAsBASE64 = &Macro{
		ReturnType: String,
		EvalPhase:  MacrosEvalPhaseD,
		VerbName:   "AsBASE64",
	}

	// Phase E – extra unused phase
)

func (m *Macro) String() string {
	// TODO maybe add args if given, so we get nicer error messages?
	return fmt.Sprintf("kubegen.%s.%s", m.ReturnType.String(), m.VerbName)
}

func MakeModifierStringJoin(c *Converter, branch *BranchLocator, _ *Macro) (ModifierCallback, error) {
	cb := func(m *Modifier, c *Converter) error {
		x := []string{}
		branch.Value().ArrayEach(func(_ int, value interface{}, dataType ValueType) error {
			x = append(x, fmt.Sprintf("%v", value))
			return nil
		})
		if err := c.Set(branch, strings.Join(x, "")); err != nil {
			return fmt.Errorf("could not join string – %v", err)
		}
		return nil
	}
	return c.TypeCheckModifier(branch, Array, cb)
}

func MakeModifierStringAsYAML(_ *Converter, _ *BranchLocator, _ *Macro) (ModifierCallback, error) {
	cb := func(m *Modifier, c *Converter) error {
		o := new(interface{})
		js, err := m.Branch.Value().BytesAsJSON()
		if err != nil {
			return err
		}
		if err := json.Unmarshal(js, o); err != nil {
			return err
		}
		x, err := yaml.Marshal(o)
		if err != nil {
			return err
		}
		{
			if err := c.Set(m.Branch, string(x)); err != nil {
				return err
			}
			return nil
		}
	}
	return cb, nil
}

func MakeModifierStringAsJSON(_ *Converter, _ *BranchLocator, _ *Macro) (ModifierCallback, error) {
	cb := func(m *Modifier, c *Converter) error {
		js, err := m.Branch.Value().StringAsJSON()
		if err != nil {
			return err
		}
		if err := c.Set(m.Branch, js); err != nil {
			return err
		}
		return nil
	}
	return cb, nil
}

func MakeModifierStringAsBASE64(_ *Converter, _ *BranchLocator, _ *Macro) (ModifierCallback, error) {
	cb := func(m *Modifier, c *Converter) error {
		data := []byte{}
		v := m.Branch.Value()
		if vt, _ := v.Check(); *vt == String {
			data = []byte(m.Branch.value.self.(string))
		} else {
			js, err := v.BytesAsJSON()
			if err != nil {
				return err
			}
			data = js
		}
		if err := c.Set(m.Branch, base64.StdEncoding.EncodeToString(data)); err != nil {
			return err
		}
		return nil
	}
	return cb, nil
}

func doLoadJSON(c *Converter, branch *BranchLocator, m *Macro, newData []byte) error {
	/*
		var (
			err         error
			oldData     []byte
			oldDataTemp []byte
			oldDataType jsonparser.ValueType
			oldObj      map[string]interface{}
			newObj      interface{}
		)

		isRoot := (len(branch.path[1:]) == 1)
		switch m.ReturnType {
		case Object:
			if isRoot {
				_, oldDataType, _, err = jsonparser.Get(c.data)
				oldData = make([]byte, len(c.data))
				copy(oldData, c.data)
			} else {
				oldDataTemp, oldDataType, _, err = jsonparser.Get(c.data, branch.parent.path[1:]...)
				oldData = make([]byte, len(oldDataTemp))
				copy(oldData, oldDataTemp)
			}
		case Array:
			if isRoot {
				return fmt.Errorf("cannot insert array in place of root object")
			}
			oldDataTemp, oldDataType, _, err = jsonparser.Get(c.data, branch.parent.path[1:]...)
			oldData = make([]byte, len(oldDataTemp))
			copy(oldData, oldDataTemp)
		}

		switch {
		case err != nil:
			return fmt.Errorf("cannot get old data – %v", err)
		case len(oldData) == 0:
			return fmt.Errorf("old data is empty")
		case oldDataType != jsonparser.Object:
			return fmt.Errorf("old data type is %s, but must be an object", oldDataType)
		}

		oldData = jsonparser.Delete(oldData, branch.path[len(branch.path)-1])

		if err := json.Unmarshal(oldData, &oldObj); err != nil {
			return fmt.Errorf("cannot unmarshal old data – %v", err)
		}

		if err := json.Unmarshal(newData, &newObj); err != nil {
			return fmt.Errorf("cannot unmarshal new data – %v", err)
		}

		switch m.ReturnType {
		case Object:
			//if err := mergo.MergeWithOverwrite(&oldObj, newObj.(map[string]interface{})); err != nil {
			if err := mergo.Merge(&oldObj, newObj.(map[string]interface{})); err != nil {
				return fmt.Errorf("cannot merge – %v", err)
			}
			if newData, err = json.Marshal(oldObj); err != nil {
				return fmt.Errorf("cannot marshal new object – %v", err)
			}
		case Array:
			if len(oldObj) > 1 {
				return fmt.Errorf("old data object contains non expected keys, cannot replace with an array")
			}
			if newData, err = json.Marshal(newObj); err != nil {
				return fmt.Errorf("cannot marshal new object – %v", err)
			}
		}

		c.Delete(branch)

		switch m.ReturnType {
		case Object:
			if isRoot {
				if c.data, err = util.EnsureJSON(newData); err != nil {
					return err
				}
				return nil
			}
		}

		if c.data, err = jsonparser.Set(c.data, newData, branch.parent.path[1:]...); err != nil {
			return fmt.Errorf("could not set %s value of %s – %v", m.ReturnType.String(), branch.parent.PathToString(), err)
		}
		if c.data, err = util.EnsureJSON(c.data); err != nil {
			return err
		}
	*/
	return nil
}

func addModifierLoadJSON(c *Converter, branch *BranchLocator, _ *Macro, jsonData []byte) (ModifierCallback, error) {
	cb := func(m *Modifier, c *Converter) error {
		return doLoadJSON(c, m.Branch, m.Macro, jsonData)
	}
	return c.TypeCheckModifier(branch, String, cb)
}

// TODO: generalise the way of passing contextual arugments - or is it better now?

func MakeArrayLoadJSON(c *Converter, branch *BranchLocator, jsonData []byte) (ModifierCallback, error) {
	return addModifierLoadJSON(c, branch, LoadArrayJSON, jsonData)
}

func MakeObjectLoadJSON(c *Converter, branch *BranchLocator, jsonData []byte) (ModifierCallback, error) {
	return addModifierLoadJSON(c, branch, LoadObjectJSON, jsonData)
}
