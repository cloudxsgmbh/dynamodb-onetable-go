/*
Package tests – shared test infrastructure.

Mirrors JS: test/utils/init.ts + test/schemas/
*/
package tests

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	ddb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	ot "github.com/cloudxsgmbh/dynamodb-onetable-go"
)

// ─── regexps ─────────────────────────────────────────────────────────────────

var (
	reULID  = regexp.MustCompile(`^[0-9A-Z]{26}$`)
	reEmail = regexp.MustCompile(`^[^@]+@[^@]+\.[^@]+$`)
)

// ─── mock helpers ─────────────────────────────────────────────────────────────

// applyUpdateExpression naively applies a DynamoDB UpdateExpression of the form
// "set #a = :a, #b = :b remove #c, #d add #e :e delete #f :f"
// Good enough for tests – no nested paths, no arithmetic, no type-safety checks.
func applyUpdateExpression(
	item map[string]types.AttributeValue,
	expr string,
	names map[string]string,
	vals map[string]types.AttributeValue,
) {
	// resolve a name token (#_0) or value token (:_0)
	resolveName := func(tok string) string {
		tok = strings.TrimSpace(tok)
		if v, ok := names[tok]; ok {
			return v
		}
		return tok
	}
	resolveVal := func(tok string) types.AttributeValue {
		tok = strings.TrimSpace(tok)
		return vals[tok]
	}

	// split into clauses: "set ...", "remove ...", "add ...", "delete ..."
	lower := strings.ToLower(expr)
	clauses := map[string]string{}
	keywords := []string{"set", "remove", "add", "delete"}
	positions := []int{}
	for _, kw := range keywords {
		// find first occurrence of the keyword as a word boundary
		idx := strings.Index(lower, kw)
		if idx >= 0 {
			positions = append(positions, idx)
		}
	}
	// sort positions
	for i := 0; i < len(positions); i++ {
		for j := i + 1; j < len(positions); j++ {
			if positions[j] < positions[i] {
				positions[i], positions[j] = positions[j], positions[i]
			}
		}
	}
	for i, pos := range positions {
		end := len(expr)
		if i+1 < len(positions) {
			end = positions[i+1]
		}
		clause := strings.TrimSpace(expr[pos:end])
		parts := strings.SplitN(clause, " ", 2)
		if len(parts) == 2 {
			clauses[strings.ToLower(parts[0])] = parts[1]
		}
	}

	// process SET
	if setClause, ok := clauses["set"]; ok {
		for _, assignment := range strings.Split(setClause, ",") {
			assignment = strings.TrimSpace(assignment)
			if assignment == "" {
				continue
			}
			eqIdx := strings.Index(assignment, "=")
			if eqIdx < 0 {
				continue
			}
			lhs := strings.TrimSpace(assignment[:eqIdx])
			rhs := strings.TrimSpace(assignment[eqIdx+1:])
			attr := resolveName(lhs)
			val := resolveVal(rhs)
			if val != nil {
				item[attr] = val
			}
		}
	}

	// process REMOVE
	if removeClause, ok := clauses["remove"]; ok {
		for _, tok := range strings.Split(removeClause, ",") {
			attr := resolveName(strings.TrimSpace(tok))
			if attr != "" {
				delete(item, attr)
			}
		}
	}

	// process ADD (numeric increment / set add — simplified)
	if addClause, ok := clauses["add"]; ok {
		for _, assignment := range strings.Split(addClause, ",") {
			assignment = strings.TrimSpace(assignment)
			parts := strings.Fields(assignment)
			if len(parts) < 2 {
				continue
			}
			attr := resolveName(parts[0])
			val := resolveVal(parts[1])
			if val != nil {
				item[attr] = val // simplified: just set
			}
		}
	}
}

// filterItems applies a FilterExpression (simplified) to a list of items.
// Handles: attr = :val, attribute_exists, attribute_not_exists, begins_with, AND/OR clauses.
func filterItems(
	items []map[string]types.AttributeValue,
	filterExpr string,
	names map[string]string,
	vals map[string]types.AttributeValue,
) []map[string]types.AttributeValue {
	if filterExpr == "" {
		return items
	}
	var out []map[string]types.AttributeValue
	for _, item := range items {
		if evalFilter(item, filterExpr, names, vals) {
			out = append(out, item)
		}
	}
	return out
}

// evalFilter evaluates a filter expression against an item.
// Supports: attr = :val, attr <> :val, attr < :val, attr <= :val, attr > :val, attr >= :val,
// attribute_exists(attr), attribute_not_exists(attr), begins_with(attr, :val),
// contains(attr, :val), AND, OR, parenthesised sub-expressions.
func evalFilter(
	item map[string]types.AttributeValue,
	expr string,
	names map[string]string,
	vals map[string]types.AttributeValue,
) bool {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return true
	}

	// strip outer parens
	if strings.HasPrefix(expr, "(") && strings.HasSuffix(expr, ")") {
		inner := expr[1 : len(expr)-1]
		if balanced(inner) {
			return evalFilter(item, inner, names, vals)
		}
	}

	// split on top-level " and " / " or "
	if parts := splitTopLevel(expr, " and "); len(parts) > 1 {
		for _, p := range parts {
			if !evalFilter(item, p, names, vals) {
				return false
			}
		}
		return true
	}
	if parts := splitTopLevel(expr, " or "); len(parts) > 1 {
		for _, p := range parts {
			if evalFilter(item, p, names, vals) {
				return true
			}
		}
		return false
	}

	expr = strings.TrimSpace(expr)
	lower := strings.ToLower(expr)

	resolveName := func(tok string) string {
		tok = strings.TrimSpace(tok)
		if v, ok := names[tok]; ok {
			return v
		}
		return tok
	}
	resolveVal := func(tok string) types.AttributeValue {
		tok = strings.TrimSpace(tok)
		return vals[tok]
	}
	getItemVal := func(attrName string) string {
		if av, ok := item[attrName]; ok {
			return avStr(av)
		}
		return ""
	}

	// attribute_exists / attribute_not_exists
	if strings.HasPrefix(lower, "attribute_not_exists(") {
		inner := strings.TrimSuffix(strings.TrimPrefix(expr, strings.ToLower(expr[:len("attribute_not_exists(")])), ")")
		attr := resolveName(strings.TrimSpace(inner))
		_, exists := item[attr]
		return !exists
	}
	if strings.HasPrefix(lower, "attribute_exists(") {
		inner := strings.TrimSuffix(strings.TrimPrefix(expr, strings.ToLower(expr[:len("attribute_exists(")])), ")")
		attr := resolveName(strings.TrimSpace(inner))
		_, exists := item[attr]
		return exists
	}

	// begins_with(attr, :val)
	if strings.HasPrefix(lower, "begins_with(") {
		inner := strings.TrimSuffix(expr[len("begins_with("):], ")")
		commIdx := strings.LastIndex(inner, ",")
		if commIdx >= 0 {
			attr := resolveName(inner[:commIdx])
			valTok := strings.TrimSpace(inner[commIdx+1:])
			prefix := avStr(resolveVal(valTok))
			return strings.HasPrefix(getItemVal(attr), prefix)
		}
	}

	// contains(attr, :val)
	if strings.HasPrefix(lower, "contains(") {
		inner := strings.TrimSuffix(expr[len("contains("):], ")")
		commIdx := strings.LastIndex(inner, ",")
		if commIdx >= 0 {
			attr := resolveName(inner[:commIdx])
			valTok := strings.TrimSpace(inner[commIdx+1:])
			needle := avStr(resolveVal(valTok))
			return strings.Contains(getItemVal(attr), needle)
		}
	}

	// comparison operators: attr OP :val
	for _, op := range []string{"<>", "<=", ">=", "<", ">", "="} {
		idx := strings.Index(expr, op)
		if idx < 0 {
			continue
		}
		lhs := strings.TrimSpace(expr[:idx])
		rhs := strings.TrimSpace(expr[idx+len(op):])
		attr := resolveName(lhs)
		itemVal := getItemVal(attr)
		expected := avStr(resolveVal(rhs))
		switch op {
		case "=":
			return itemVal == expected
		case "<>":
			return itemVal != expected
		case "<":
			return itemVal < expected
		case "<=":
			return itemVal <= expected
		case ">":
			return itemVal > expected
		case ">=":
			return itemVal >= expected
		}
	}

	return true // unknown expression — pass through
}

// balanced reports whether the parentheses in s are balanced.
func balanced(s string) bool {
	depth := 0
	for _, c := range s {
		if c == '(' {
			depth++
		} else if c == ')' {
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

// splitTopLevel splits expr on sep only at depth 0 (not inside parens).
func splitTopLevel(expr, sep string) []string {
	lower := strings.ToLower(expr)
	var parts []string
	depth := 0
	last := 0
	for i := 0; i < len(lower); i++ {
		switch lower[i] {
		case '(':
			depth++
		case ')':
			depth--
		}
		if depth == 0 && strings.HasPrefix(lower[i:], sep) {
			parts = append(parts, strings.TrimSpace(expr[last:i]))
			last = i + len(sep)
			i += len(sep) - 1
		}
	}
	parts = append(parts, strings.TrimSpace(expr[last:]))
	return parts
}

// conditionPasses evaluates a condition expression against an item.
// Uses evalFilter for full expression support.
func conditionPasses(
	item map[string]types.AttributeValue,
	condExpr string,
	names map[string]string,
	vals ...map[string]types.AttributeValue,
) bool {
	if condExpr == "" {
		return true
	}
	var v map[string]types.AttributeValue
	if len(vals) > 0 {
		v = vals[0]
	}
	return evalFilter(item, condExpr, names, v)
}

func isULID(s string) bool  { return reULID.MatchString(s) }
func isEmail(s string) bool { return reEmail.MatchString(s) }

// ─── fullMock ─────────────────────────────────────────────────────────────────

// fullMock is a thread-safe in-memory DynamoDB substitute.
type fullMock struct {
	mu     sync.RWMutex
	tables map[string]map[string]map[string]types.AttributeValue
}

func newFullMock() *fullMock {
	return &fullMock{tables: map[string]map[string]map[string]types.AttributeValue{}}
}

func (m *fullMock) tbl(name string) map[string]map[string]types.AttributeValue {
	if m.tables[name] == nil {
		m.tables[name] = map[string]map[string]types.AttributeValue{}
	}
	return m.tables[name]
}

func avStr(av types.AttributeValue) string {
	switch v := av.(type) {
	case *types.AttributeValueMemberS:
		return v.Value
	case *types.AttributeValueMemberN:
		return v.Value
	}
	return ""
}

func itemKey(item map[string]types.AttributeValue) string {
	return avStr(item["pk"]) + "||" + avStr(item["sk"])
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (m *fullMock) PutItem(_ context.Context, p *ddb.PutItemInput, _ ...func(*ddb.Options)) (*ddb.PutItemOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := m.tbl(deref(p.TableName))
	k := itemKey(p.Item)
	cond := deref(p.ConditionExpression)
	if cond != "" {
		existing := t[k]
		if existing == nil {
			existing = map[string]types.AttributeValue{}
		}
		if !conditionPasses(existing, cond, p.ExpressionAttributeNames, p.ExpressionAttributeValues) {
			return nil, fmt.Errorf("ConditionalCheckFailedException: condition not met")
		}
	}
	t[k] = p.Item
	return &ddb.PutItemOutput{}, nil
}

func (m *fullMock) GetItem(_ context.Context, p *ddb.GetItemInput, _ ...func(*ddb.Options)) (*ddb.GetItemOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	item := m.tbl(deref(p.TableName))[itemKey(p.Key)]
	return &ddb.GetItemOutput{Item: item}, nil
}

func (m *fullMock) DeleteItem(_ context.Context, p *ddb.DeleteItemInput, _ ...func(*ddb.Options)) (*ddb.DeleteItemOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := m.tbl(deref(p.TableName))
	k := itemKey(p.Key)
	prior := t[k]
	delete(t, k)
	return &ddb.DeleteItemOutput{Attributes: prior}, nil
}

func (m *fullMock) UpdateItem(_ context.Context, p *ddb.UpdateItemInput, _ ...func(*ddb.Options)) (*ddb.UpdateItemOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := m.tbl(deref(p.TableName))
	k := itemKey(p.Key)
	existing := t[k]
	if existing == nil {
		existing = map[string]types.AttributeValue{}
	}
	// check condition
	cond := deref(p.ConditionExpression)
	if cond != "" && !conditionPasses(existing, cond, p.ExpressionAttributeNames, p.ExpressionAttributeValues) {
		return nil, fmt.Errorf("ConditionalCheckFailedException: condition not met for update")
	}
	// merge key back
	for kk, vv := range p.Key {
		existing[kk] = vv
	}
	// apply UpdateExpression
	if p.UpdateExpression != nil {
		applyUpdateExpression(existing, deref(p.UpdateExpression), p.ExpressionAttributeNames, p.ExpressionAttributeValues)
	}
	t[k] = existing
	return &ddb.UpdateItemOutput{Attributes: existing}, nil
}

func (m *fullMock) Query(_ context.Context, p *ddb.QueryInput, _ ...func(*ddb.Options)) (*ddb.QueryOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var all []map[string]types.AttributeValue
	for _, v := range m.tbl(deref(p.TableName)) {
		all = append(all, v)
	}
	// apply KeyConditionExpression + FilterExpression combined
	combined := ""
	if p.KeyConditionExpression != nil && *p.KeyConditionExpression != "" {
		combined = *p.KeyConditionExpression
	}
	if p.FilterExpression != nil && *p.FilterExpression != "" {
		if combined != "" {
			combined += " and " + *p.FilterExpression
		} else {
			combined = *p.FilterExpression
		}
	}
	items := filterItems(all, combined, p.ExpressionAttributeNames, p.ExpressionAttributeValues)
	return &ddb.QueryOutput{Items: items, Count: int32(len(items))}, nil
}

func (m *fullMock) Scan(_ context.Context, p *ddb.ScanInput, _ ...func(*ddb.Options)) (*ddb.ScanOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var all []map[string]types.AttributeValue
	for _, v := range m.tbl(deref(p.TableName)) {
		all = append(all, v)
	}
	items := filterItems(all, deref(p.FilterExpression), p.ExpressionAttributeNames, p.ExpressionAttributeValues)
	return &ddb.ScanOutput{Items: items, Count: int32(len(items)), ScannedCount: int32(len(all))}, nil
}

func (m *fullMock) BatchGetItem(_ context.Context, p *ddb.BatchGetItemInput, _ ...func(*ddb.Options)) (*ddb.BatchGetItemOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	resp := map[string][]map[string]types.AttributeValue{}
	for tblName, keysAndAttrs := range p.RequestItems {
		for _, key := range keysAndAttrs.Keys {
			if item := m.tbl(tblName)[itemKey(key)]; item != nil {
				resp[tblName] = append(resp[tblName], item)
			}
		}
	}
	return &ddb.BatchGetItemOutput{Responses: resp}, nil
}

func (m *fullMock) BatchWriteItem(_ context.Context, p *ddb.BatchWriteItemInput, _ ...func(*ddb.Options)) (*ddb.BatchWriteItemOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for tblName, reqs := range p.RequestItems {
		for _, req := range reqs {
			if req.PutRequest != nil {
				m.tbl(tblName)[itemKey(req.PutRequest.Item)] = req.PutRequest.Item
			} else if req.DeleteRequest != nil {
				delete(m.tbl(tblName), itemKey(req.DeleteRequest.Key))
			}
		}
	}
	return &ddb.BatchWriteItemOutput{}, nil
}

func (m *fullMock) TransactGetItems(_ context.Context, p *ddb.TransactGetItemsInput, _ ...func(*ddb.Options)) (*ddb.TransactGetItemsOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var responses []types.ItemResponse
	for _, ti := range p.TransactItems {
		if ti.Get != nil {
			item := m.tbl(deref(ti.Get.TableName))[itemKey(ti.Get.Key)]
			responses = append(responses, types.ItemResponse{Item: item})
		}
	}
	return &ddb.TransactGetItemsOutput{Responses: responses}, nil
}

func (m *fullMock) TransactWriteItems(_ context.Context, p *ddb.TransactWriteItemsInput, _ ...func(*ddb.Options)) (*ddb.TransactWriteItemsOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// first pass: check all conditions atomically
	for _, ti := range p.TransactItems {
		switch {
		case ti.Put != nil:
			cond := deref(ti.Put.ConditionExpression)
			if cond != "" {
				tbl := m.tbl(deref(ti.Put.TableName))
				existing := tbl[itemKey(ti.Put.Item)]
				if existing == nil {
					existing = map[string]types.AttributeValue{}
				}
				if !conditionPasses(existing, cond, ti.Put.ExpressionAttributeNames, ti.Put.ExpressionAttributeValues) {
					return nil, fmt.Errorf("TransactionCanceledException: condition failed for Put")
				}
			}
		case ti.Update != nil:
			cond := deref(ti.Update.ConditionExpression)
			if cond != "" {
				tbl := m.tbl(deref(ti.Update.TableName))
				existing := tbl[itemKey(ti.Update.Key)]
				if existing == nil {
					existing = map[string]types.AttributeValue{}
				}
				if !conditionPasses(existing, cond, ti.Update.ExpressionAttributeNames, ti.Update.ExpressionAttributeValues) {
					return nil, fmt.Errorf("TransactionCanceledException: condition failed for Update")
				}
			}
		case ti.Delete != nil:
			cond := deref(ti.Delete.ConditionExpression)
			if cond != "" {
				tbl := m.tbl(deref(ti.Delete.TableName))
				existing := tbl[itemKey(ti.Delete.Key)]
				if existing == nil {
					existing = map[string]types.AttributeValue{}
				}
				if !conditionPasses(existing, cond, ti.Delete.ExpressionAttributeNames, ti.Delete.ExpressionAttributeValues) {
					return nil, fmt.Errorf("TransactionCanceledException: condition failed for Delete")
				}
			}
		}
	}
	// second pass: apply
	for _, ti := range p.TransactItems {
		switch {
		case ti.Put != nil:
			m.tbl(deref(ti.Put.TableName))[itemKey(ti.Put.Item)] = ti.Put.Item
		case ti.Delete != nil:
			delete(m.tbl(deref(ti.Delete.TableName)), itemKey(ti.Delete.Key))
		case ti.Update != nil:
			t := m.tbl(deref(ti.Update.TableName))
			k := itemKey(ti.Update.Key)
			existing := t[k]
			if existing == nil {
				existing = map[string]types.AttributeValue{}
			}
			for kk, vv := range ti.Update.Key {
				existing[kk] = vv
			}
			if ti.Update.UpdateExpression != nil {
				applyUpdateExpression(existing, deref(ti.Update.UpdateExpression),
					ti.Update.ExpressionAttributeNames, ti.Update.ExpressionAttributeValues)
			}
			t[k] = existing
		}
	}
	return &ddb.TransactWriteItemsOutput{}, nil
}

func (m *fullMock) CreateTable(_ context.Context, p *ddb.CreateTableInput, _ ...func(*ddb.Options)) (*ddb.CreateTableOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	name := deref(p.TableName)
	if m.tables[name] == nil {
		m.tables[name] = map[string]map[string]types.AttributeValue{}
	}
	return &ddb.CreateTableOutput{}, nil
}

func (m *fullMock) DeleteTable(_ context.Context, p *ddb.DeleteTableInput, _ ...func(*ddb.Options)) (*ddb.DeleteTableOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tables, deref(p.TableName))
	return &ddb.DeleteTableOutput{}, nil
}

func (m *fullMock) UpdateTable(_ context.Context, _ *ddb.UpdateTableInput, _ ...func(*ddb.Options)) (*ddb.UpdateTableOutput, error) {
	return &ddb.UpdateTableOutput{}, nil
}

func (m *fullMock) DescribeTable(_ context.Context, _ *ddb.DescribeTableInput, _ ...func(*ddb.Options)) (*ddb.DescribeTableOutput, error) {
	return &ddb.DescribeTableOutput{}, nil
}

func (m *fullMock) ListTables(_ context.Context, _ *ddb.ListTablesInput, _ ...func(*ddb.Options)) (*ddb.ListTablesOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.tables))
	for n := range m.tables {
		names = append(names, n)
	}
	return &ddb.ListTablesOutput{TableNames: names}, nil
}

func (m *fullMock) UpdateTimeToLive(_ context.Context, _ *ddb.UpdateTimeToLiveInput, _ ...func(*ddb.Options)) (*ddb.UpdateTimeToLiveOutput, error) {
	return &ddb.UpdateTimeToLiveOutput{}, nil
}

func (m *fullMock) count(table string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.tbl(table))
}

// ─── schema definitions ───────────────────────────────────────────────────────

var DefaultSchema = &ot.SchemaDef{
	Format:  "onetable:1.1.0",
	Version: "0.0.1",
	Indexes: map[string]*ot.IndexDef{
		"primary": {Hash: "pk", Sort: "sk"},
		"gs1":     {Hash: "gs1pk", Sort: "gs1sk", Project: "all"},
		"gs2":     {Hash: "gs2pk", Sort: "gs2sk", Project: "all"},
		"gs3":     {Hash: "gs3pk", Sort: "gs3sk", Project: "all"},
	},
	Models: map[string]ot.ModelDef{
		"User": {
			"pk":         {Type: ot.FieldTypeString, Value: "${_type}#${id}"},
			"sk":         {Type: ot.FieldTypeString, Value: "${_type}#"},
			"id":         {Type: ot.FieldTypeString, Generate: "ulid"},
			"name":       {Type: ot.FieldTypeString},
			"email":      {Type: ot.FieldTypeString},
			"status":     {Type: ot.FieldTypeString, Default: "idle"},
			"age":        {Type: ot.FieldTypeNumber},
			"profile":    {Type: ot.FieldTypeObject},
			"registered": {Type: ot.FieldTypeDate},
			"gs1pk":      {Type: ot.FieldTypeString, Value: "${_type}#${name}"},
			"gs1sk":      {Type: ot.FieldTypeString, Value: "${_type}#"},
			"gs2pk":      {Type: ot.FieldTypeString, Value: "type:${_type}"},
			"gs2sk":      {Type: ot.FieldTypeString, Value: "${_type}#${id}"},
			"gs3pk":      {Type: ot.FieldTypeString, Value: "${_type}#${status}"},
			"gs3sk":      {Type: ot.FieldTypeString, Value: "${_type}#${name}"},
		},
		"Pet": {
			"pk":    {Type: ot.FieldTypeString, Value: "${_type}"},
			"sk":    {Type: ot.FieldTypeString, Value: "${_type}#${id}"},
			"id":    {Type: ot.FieldTypeString, Generate: "ulid"},
			"name":  {Type: ot.FieldTypeString},
			"race":  {Type: ot.FieldTypeString, Enum: []string{"dog", "cat", "fish"}, Required: true},
			"breed": {Type: ot.FieldTypeString, Required: true},
		},
	},
	Params: &ot.SchemaParams{IsoDates: true, Timestamps: true},
}

var ValidationSchema = &ot.SchemaDef{
	Format:  "onetable:1.1.0",
	Version: "0.0.1",
	Indexes: map[string]*ot.IndexDef{"primary": {Hash: "pk", Sort: "sk"}},
	Models: map[string]ot.ModelDef{
		"User": {
			"pk":      {Type: ot.FieldTypeString, Value: "user#${id}"},
			"sk":      {Type: ot.FieldTypeString, Value: "user#"},
			"id":      {Type: ot.FieldTypeString, Generate: "ulid"},
			"name":    {Type: ot.FieldTypeString, Required: true, Validate: `/^[a-zA-Z' ]+$/`},
			"email":   {Type: ot.FieldTypeString, Required: true, Validate: `/^[^@]+@[^@]+\.[^@]+$/`},
			"address": {Type: ot.FieldTypeString, Validate: `/^[a-zA-Z0-9 .,'-]+$/`},
			"city":    {Type: ot.FieldTypeString, Validate: "San Francisco"},
			"zip":     {Type: ot.FieldTypeString, Validate: `/^[a-z0-9 ,.-]+$/`},
			"phone":   {Type: ot.FieldTypeString, Validate: `/^[ 0-9-()+]+$/`},
			"status":  {Type: ot.FieldTypeString, Required: true},
			"age":     {Type: ot.FieldTypeNumber},
		},
	},
	Params: &ot.SchemaParams{},
}

var NestedSchema = &ot.SchemaDef{
	Version: "0.0.1",
	Indexes: map[string]*ot.IndexDef{"primary": {Hash: "pk", Sort: "sk"}},
	Models: map[string]ot.ModelDef{
		"User": {
			"pk":      {Type: ot.FieldTypeString, Value: "${_type}#${id}"},
			"sk":      {Type: ot.FieldTypeString, Value: "${_type}#"},
			"id":      {Type: ot.FieldTypeString, Generate: "ulid"},
			"name":    {Type: ot.FieldTypeString, Required: true},
			"email":   {Type: ot.FieldTypeString},
			"status":  {Type: ot.FieldTypeString},
			"balance": {Type: ot.FieldTypeNumber},
			"tokens":  {Type: ot.FieldTypeArray},
			"started": {Type: ot.FieldTypeDate},
			"location": {
				Type: ot.FieldTypeObject,
				Schema: ot.FieldMap{
					"address": {Type: ot.FieldTypeString},
					"city":    {Type: ot.FieldTypeString},
					"zip":     {Type: ot.FieldTypeString},
					"started": {Type: ot.FieldTypeDate},
				},
			},
		},
	},
	Params: &ot.SchemaParams{Timestamps: true},
}

var MappedSchema = &ot.SchemaDef{
	Format:  "onetable:1.1.0",
	Version: "0.0.1",
	Indexes: map[string]*ot.IndexDef{
		"primary": {Hash: "pk", Sort: "sk"},
		"gs1":     {Hash: "pk1", Sort: "sk1", Project: []string{"pk1", "sk1", "data"}},
	},
	Models: map[string]ot.ModelDef{
		"User": {
			"primaryHash": {Type: ot.FieldTypeString, Value: "us#${id}", Map: "pk"},
			"primarySort": {Type: ot.FieldTypeString, Value: "us#", Map: "sk"},
			"id":          {Type: ot.FieldTypeString, Generate: "ulid"},
			"name":        {Type: ot.FieldTypeString, Map: "nm"},
			"email":       {Type: ot.FieldTypeString, Map: "em"},
			"status":      {Type: ot.FieldTypeString, Map: "st"},
			"address":     {Type: ot.FieldTypeString, Map: "data.address"},
			"city":        {Type: ot.FieldTypeString, Map: "data.city"},
			"zip":         {Type: ot.FieldTypeString, Map: "data.zip"},
			"gs1pk":       {Type: ot.FieldTypeString, Value: "ty#us", Map: "pk1"},
			"gs1sk":       {Type: ot.FieldTypeString, Value: "us#${email}", Map: "sk1"},
		},
	},
	Params: &ot.SchemaParams{},
}

var TenantSchema = &ot.SchemaDef{
	Format:  "onetable:1.1.0",
	Version: "0.0.1",
	Indexes: map[string]*ot.IndexDef{
		"primary": {Hash: "pk", Sort: "sk"},
		"gs1":     {Hash: "gs1pk", Sort: "gs1sk", Project: "all"},
	},
	Models: map[string]ot.ModelDef{
		"Account": {
			"pk":    {Type: ot.FieldTypeString, Value: "${_type}#${id}"},
			"sk":    {Type: ot.FieldTypeString, Value: "${_type}#"},
			"id":    {Type: ot.FieldTypeString, Generate: "ulid"},
			"name":  {Type: ot.FieldTypeString, Required: true},
			"gs1pk": {Type: ot.FieldTypeString, Value: "${_type}#${name}"},
			"gs1sk": {Type: ot.FieldTypeString, Value: "${_type}#"},
		},
		"User": {
			"pk":        {Type: ot.FieldTypeString, Value: "Account#${accountId}"},
			"sk":        {Type: ot.FieldTypeString, Value: "${_type}#${id}"},
			"accountId": {Type: ot.FieldTypeString},
			"id":        {Type: ot.FieldTypeString, Generate: "ulid"},
			"name":      {Type: ot.FieldTypeString, Required: true},
			"email":     {Type: ot.FieldTypeString, Required: true},
			"gs1pk":     {Type: ot.FieldTypeString, Value: "${_type}#${email}"},
			"gs1sk":     {Type: ot.FieldTypeString, Value: "${_type}#${accountId}"},
		},
	},
	Params: &ot.SchemaParams{},
}

var UniqueSchema = &ot.SchemaDef{
	Format:  "onetable:1.1.0",
	Version: "0.0.1",
	Indexes: map[string]*ot.IndexDef{"primary": {Hash: "pk", Sort: "sk"}},
	Models: map[string]ot.ModelDef{
		"User": {
			"pk":           {Type: ot.FieldTypeString, Value: "${_type}#${name}"},
			"sk":           {Type: ot.FieldTypeString, Value: "${_type}#"},
			"name":         {Type: ot.FieldTypeString},
			"email":        {Type: ot.FieldTypeString, Unique: true, Required: true},
			"phone":        {Type: ot.FieldTypeString, Unique: true},
			"age":          {Type: ot.FieldTypeNumber},
			"interpolated": {Type: ot.FieldTypeString, Value: "${name}#${email}", Unique: true},
		},
	},
	Params: &ot.SchemaParams{},
}

var TimestampsSchema = &ot.SchemaDef{
	Version: "0.0.1",
	Indexes: map[string]*ot.IndexDef{"primary": {Hash: "pk", Sort: "sk"}},
	Models: map[string]ot.ModelDef{
		"User": {
			"pk":    {Type: ot.FieldTypeString, Value: "${_type}#${id}"},
			"sk":    {Type: ot.FieldTypeString, Value: "${_type}#"},
			"id":    {Type: ot.FieldTypeString, Generate: "ulid"},
			"name":  {Type: ot.FieldTypeString},
			"email": {Type: ot.FieldTypeString},
		},
	},
	Params: &ot.SchemaParams{
		Timestamps:   true,
		CreatedField: "createdAt",
		UpdatedField: "updatedAt",
	},
}

var ArraySchema = &ot.SchemaDef{
	Format:  "onetable:1.1.0",
	Version: "0.0.1",
	Indexes: map[string]*ot.IndexDef{"primary": {Hash: "pk", Sort: "sk"}},
	Models: map[string]ot.ModelDef{
		"User": {
			"pk":    {Type: ot.FieldTypeString, Value: "${_type}#${email}"},
			"sk":    {Type: ot.FieldTypeString, Value: "${_type}#"},
			"email": {Type: ot.FieldTypeString, Required: true},
			"addresses": {
				Type:    ot.FieldTypeArray,
				Default: []any{},
				Items: &ot.ItemsDef{
					Schema: ot.FieldMap{
						"street": {Type: ot.FieldTypeString},
						"zip":    {Type: ot.FieldTypeNumber},
					},
				},
			},
		},
	},
}

var PartialSchema = &ot.SchemaDef{
	Format:  "onetable:1.1.0",
	Version: "0.0.1",
	Indexes: map[string]*ot.IndexDef{
		"primary": {Hash: "pk", Sort: "sk", Project: "all"},
		"gs1":     {Hash: "gs1pk", Sort: "gs1sk", Project: "all"},
	},
	Models: map[string]ot.ModelDef{
		"User": {
			"pk":     {Type: ot.FieldTypeString, Value: "${_type}#${id}"},
			"sk":     {Type: ot.FieldTypeString, Value: "${_type}#"},
			"id":     {Type: ot.FieldTypeString, Required: true, Generate: "ulid"},
			"email":  {Type: ot.FieldTypeString, Required: true},
			"status": {Type: ot.FieldTypeString, Required: true, Default: "active"},
			"address": {
				Type: ot.FieldTypeObject,
				Schema: ot.FieldMap{
					"street": {Type: ot.FieldTypeString},
					"zip":    {Type: ot.FieldTypeNumber},
					"box": {
						Type:    ot.FieldTypeObject,
						Default: map[string]any{},
						Schema: ot.FieldMap{
							"start": {Type: ot.FieldTypeDate},
							"end":   {Type: ot.FieldTypeDate},
						},
					},
				},
			},
		},
	},
	Params: &ot.SchemaParams{},
}

// ─── table factory ────────────────────────────────────────────────────────────

func makeTable(t *testing.T, name string, schema *ot.SchemaDef, partial bool) (*ot.Table, *fullMock) {
	t.Helper()
	mock := newFullMock()
	mock.tables[name] = map[string]map[string]types.AttributeValue{}
	tbl, err := ot.NewTable(ot.TableParams{
		Name:    name,
		Client:  mock,
		Schema:  schema,
		Partial: partial,
	})
	if err != nil {
		t.Fatalf("NewTable %q: %v", name, err)
	}
	return tbl, mock
}

// ─── assertion helpers ────────────────────────────────────────────────────────

func assertStr(t *testing.T, item ot.Item, key, want string) {
	t.Helper()
	got := fmt.Sprintf("%v", item[key])
	if got != want {
		t.Errorf("item[%q] = %q, want %q", key, got, want)
	}
}

func assertNum(t *testing.T, item ot.Item, key string, want float64) {
	t.Helper()
	switch v := item[key].(type) {
	case float64:
		if v != want {
			t.Errorf("item[%q] = %v, want %v", key, v, want)
		}
	case int:
		if float64(v) != want {
			t.Errorf("item[%q] = %v, want %v", key, v, want)
		}
	default:
		t.Errorf("item[%q] type %T = %v, want float64(%v)", key, item[key], item[key], want)
	}
}

func assertULID(t *testing.T, v any) {
	t.Helper()
	s, ok := v.(string)
	if !ok || !isULID(s) {
		t.Errorf("expected ULID, got %T(%v)", v, v)
	}
}

func assertDate(t *testing.T, v any) {
	t.Helper()
	if _, ok := v.(time.Time); !ok {
		t.Errorf("expected time.Time, got %T: %v", v, v)
	}
}

func assertAbsent(t *testing.T, item ot.Item, key string) {
	t.Helper()
	if _, exists := item[key]; exists {
		t.Errorf("expected item[%q] absent, got %v", key, item[key])
	}
}

func assertPresent(t *testing.T, item ot.Item, key string) {
	t.Helper()
	if item[key] == nil {
		t.Errorf("expected item[%q] defined", key)
	}
}

func assertNil(t *testing.T, item ot.Item) {
	t.Helper()
	if item != nil {
		t.Errorf("expected nil item, got %v", item)
	}
}

// toAnySlice converts []any, []map[string]any etc. to []any.
func toAnySlice(v any) []any {
	switch s := v.(type) {
	case []any:
		return s
	case []map[string]any:
		out := make([]any, len(s))
		for i, m := range s {
			out[i] = m
		}
		return out
	}
	return nil
}

func assertLen(t *testing.T, items []ot.Item, want int) {
	t.Helper()
	if len(items) != want {
		t.Errorf("expected %d items, got %d", want, len(items))
	}
}

func assertContains(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Errorf("%q does not contain %q", s, sub)
	}
}

func assertErrCode(t *testing.T, err error, code ot.ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %q, got nil", code)
	}
	var ote *ot.OneTableError
	if e, ok := err.(*ot.OneTableError); ok {
		ote = e
	}
	if ote == nil || ote.Code != code {
		t.Errorf("expected error code %q, got: %v", code, err)
	}
}

func bg() context.Context { return context.Background() }

// storeRaw marshals a plain Item and stores it in the mock at the given table.
func storeRaw(mock *fullMock, table string, item ot.Item) {
	av, _ := attributevalue.MarshalMap(item)
	mock.mu.Lock()
	mock.tbl(table)[itemKey(av)] = av
	mock.mu.Unlock()
}

// boolPtr returns a pointer to a bool.
func boolPtr(b bool) *bool { return &b }
