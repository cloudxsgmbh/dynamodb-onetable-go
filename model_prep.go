/*
Package onetable – model preparation (field parsing, ordering, mapping).

Mirrors the JS Model.prepModel / orderFields / checkType / getIndexProperties logic.
*/
package onetable

import (
	"regexp"
	"strings"
)

// prepModel builds a fieldBlock from a raw FieldMap (schema definition).
// parent is non-nil when processing a nested schema.
func (m *Model) prepModel(schemaFields FieldMap, block *fieldBlock, parent *preparedField) {
	if parent == nil {
		// Top-level: inject _type, created, updated if absent
		if _, ok := schemaFields[m.typeField]; !ok {
			hidden := true
			schemaFields[m.typeField] = &FieldDef{
				Type:   FieldTypeString,
				Hidden: &hidden,
			}
			if !m.generic {
				schemaFields[m.typeField].Required = true
			}
		}
		ts := m.table.timestamps
		if ts == true || ts == "create" {
			if _, ok := schemaFields[m.createdField]; !ok {
				schemaFields[m.createdField] = &FieldDef{Type: FieldTypeDate}
			}
		}
		if ts == true || ts == "update" {
			if _, ok := schemaFields[m.updatedField]; !ok {
				schemaFields[m.updatedField] = &FieldDef{Type: FieldTypeDate}
			}
		}
	}

	primary := m.indexes["primary"]

	// mapTargets tracks sub-props already mapped into a packed attribute
	mapTargets := map[string][]string{}

	for name, def := range schemaFields {
		if def.Type == "" {
			def.Type = FieldTypeString
			logError(m.table.log, "Missing type field for "+name, nil)
		}

		ft, err := checkType(def.Type, name, m.Name)
		if err != nil {
			panic(err.Error())
		}
		def.Type = ft

		pf := &preparedField{
			Name:          name,
			Def:           def,
			Type:          ft,
			Required:      def.Required,
			ValueTemplate: def.Value,
		}

		// isoDates: field override → table default
		if def.IsoDates != nil {
			pf.IsoDates = *def.IsoDates
		} else {
			pf.IsoDates = m.table.isoDates
		}

		// nulls
		if def.Nulls != nil {
			pf.Nulls = *def.Nulls
		} else {
			pf.Nulls = m.table.nulls
		}

		// partial – keep as pointer so we can detect "not set"
		pf.Partial = def.Partial

		// hidden: value templates are hidden by default
		if def.Hidden != nil {
			pf.Hidden = *def.Hidden
		} else if def.Value != "" {
			pf.Hidden = true
		}

		// attribute mapping
		if def.Map != "" {
			parts := strings.SplitN(def.Map, ".", 2)
			att := parts[0]
			if len(parts) == 2 {
				sub := parts[1]
				pf.Attribute = []string{att, sub}
				mapTargets[att] = append(mapTargets[att], sub)
			} else {
				pf.Attribute = []string{att}
				mapTargets[att] = append(mapTargets[att], "")
			}
		} else {
			pf.Attribute = []string{name}
		}

		// index membership
		if parent == nil {
			att := pf.Attribute[0]
			if idxName, ok := m.indexProperties[att]; ok {
				pf.IsIndexed = true
				if len(pf.Attribute) > 1 {
					panic(NewArgError("Cannot map property \"" + name + "\" to a compound attribute").Error())
				}
				if idxName == "primary" {
					pf.IsPrimary = true
					pf.Required = true
					if att == primary.Hash {
						m.hash = att
					} else if att == primary.Sort {
						m.sort = att
					}
				}
			}
		}

		// nested schema
		if def.Items != nil && ft == FieldTypeArray {
			def.Schema = def.Items.Schema
			pf.IsArray = true
		}
		if def.Schema != nil {
			if ft == FieldTypeObject || ft == FieldTypeArray {
				sub := &fieldBlock{Fields: map[string]*preparedField{}, Deps: nil}
				m.prepModel(def.Schema, sub, pf)
				pf.Block = sub
				m.nested = true
			} else {
				panic(NewArgError("Nested schema only supported for object/array fields, not \"" +
					string(ft) + "\" for field \"" + name + "\"").Error())
			}
		}

		block.Fields[name] = pf
	}

	m.mappings = mapTargets

	// mark unique fields
	for _, pf := range block.Fields {
		if pf.Def.Unique && len(pf.Attribute) == 1 && pf.Attribute[0] != m.hash && pf.Attribute[0] != m.sort {
			m.hasUniqueFields = true
		}
	}

	// topological ordering for template evaluation
	for _, pf := range block.Fields {
		m.orderFields(block, pf)
	}
}

// checkType normalises and validates the FieldType.
func checkType(t FieldType, fieldName, modelName string) (FieldType, error) {
	norm := FieldType(strings.ToLower(string(t)))
	if !validFieldTypes[norm] {
		return "", NewArgError("Unknown type \"" + string(t) + "\" for field \"" + fieldName + "\" in model \"" + modelName + "\"")
	}
	return norm, nil
}

// orderFields does a topological sort of value-template dependencies so that
// templates can safely reference other template fields.
func (m *Model) orderFields(block *fieldBlock, field *preparedField) {
	// already in deps?
	for _, d := range block.Deps {
		if d.Name == field.Name {
			return
		}
	}
	if field.ValueTemplate != "" {
		vars := getTemplateVars(field.ValueTemplate)
		for _, path := range vars {
			name := strings.Split(path, ".")[0]
			name = strings.Split(name, "[")[0]
			if ref, ok := block.Fields[name]; ok && ref != field {
				if ref.Block != nil {
					m.orderFields(ref.Block, ref)
				} else if ref.ValueTemplate != "" {
					m.orderFields(block, ref)
				}
			}
		}
	}
	block.Deps = append(block.Deps, field)
}

// getTemplateVars extracts all ${varName} references from a value template.
func getTemplateVars(tmpl string) []string {
	re := regexp.MustCompile(`\$\{(.*?)\}`)
	matches := re.FindAllStringSubmatch(tmpl, -1)
	vars := make([]string, 0, len(matches))
	for _, m := range matches {
		vars = append(vars, m[1])
	}
	return vars
}

// getIndexProperties builds a map of attribute-name → index-name for all indexes.
// Primary takes precedence over GSI/LSI when an attribute appears in both.
func getIndexProperties(indexes map[string]*IndexDef) map[string]string {
	props := map[string]string{}
	for idxName, idx := range indexes {
		for _, attr := range []string{idx.Hash, idx.Sort} {
			if attr == "" {
				continue
			}
			if props[attr] != "primary" {
				props[attr] = idxName
			}
		}
	}
	return props
}
