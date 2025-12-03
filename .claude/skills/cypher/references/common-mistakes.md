# Common Cypher Mistakes and How to Avoid Them

## Clause Ordering Errors

### MATCH After CREATE Without WITH

**Error**: `Invalid input 'MATCH': expected ... WITH`

```cypher
// WRONG
CREATE (e:Event {id: $id})
MATCH (ref:Event {id: $refId})  // ERROR!
CREATE (e)-[:REFERENCES]->(ref)

// CORRECT - Use WITH to transition
CREATE (e:Event {id: $id})
WITH e
MATCH (ref:Event {id: $refId})
CREATE (e)-[:REFERENCES]->(ref)
```

**Rule**: After CREATE, you must use WITH before MATCH.

### WHERE After WITH Without Carrying Variables

**Error**: `Variable 'x' not defined`

```cypher
// WRONG - 'a' is lost
MATCH (a:Author), (e:Event)
WITH e
WHERE a.pubkey = $pubkey  // ERROR: 'a' not in scope

// CORRECT - Include all needed variables
MATCH (a:Author), (e:Event)
WITH a, e
WHERE a.pubkey = $pubkey
```

**Rule**: WITH resets the scope. Include all variables you need.

### ORDER BY Without Aliased Return

**Error**: `Invalid input 'ORDER': expected ... AS`

```cypher
// WRONG in some contexts
RETURN n.name
ORDER BY n.name

// SAFER - Use alias
RETURN n.name AS name
ORDER BY name
```

## MERGE Mistakes

### MERGE on Complex Pattern Creates Duplicates

```cypher
// DANGEROUS - May create duplicate nodes
MERGE (a:Person {name: 'Alice'})-[:KNOWS]->(b:Person {name: 'Bob'})

// CORRECT - MERGE nodes separately first
MERGE (a:Person {name: 'Alice'})
MERGE (b:Person {name: 'Bob'})
MERGE (a)-[:KNOWS]->(b)
```

**Rule**: MERGE simple patterns, not complex ones.

### MERGE Without Unique Property

```cypher
// DANGEROUS - Will keep creating nodes
MERGE (p:Person)  // No unique identifier!
SET p.name = 'Alice'

// CORRECT - Provide unique key
MERGE (p:Person {email: $email})
SET p.name = 'Alice'
```

**Rule**: MERGE must have properties that uniquely identify the node.

### Missing ON CREATE/ON MATCH

```cypher
// LOSES context of whether new or existing
MERGE (p:Person {id: $id})
SET p.updated_at = timestamp()  // Always runs

// BETTER - Handle each case
MERGE (p:Person {id: $id})
ON CREATE SET p.created_at = timestamp()
ON MATCH SET p.updated_at = timestamp()
```

## NULL Handling Errors

### Comparing with NULL

```cypher
// WRONG - NULL = NULL is NULL, not true
WHERE n.email = null  // Never matches!

// CORRECT
WHERE n.email IS NULL
WHERE n.email IS NOT NULL
```

### NULL in Aggregations

```cypher
// count(NULL) returns 0, collect(NULL) includes NULL
MATCH (n:Person)
OPTIONAL MATCH (n)-[:BOUGHT]->(p:Product)
RETURN n.name, count(p)  // count ignores NULL
```

### NULL Propagation in Expressions

```cypher
// Any operation with NULL returns NULL
WHERE n.age + 1 > 21  // If n.age is NULL, whole expression is NULL (falsy)

// Handle with coalesce
WHERE coalesce(n.age, 0) + 1 > 21
```

## List and IN Clause Errors

### Empty List in IN

```cypher
// An empty list never matches
WHERE n.kind IN []  // Always false

// Check for empty list in application code before query
// Or use CASE:
WHERE CASE WHEN size($kinds) > 0 THEN n.kind IN $kinds ELSE true END
```

### IN with NULL Values

```cypher
// NULL in the list causes issues
WHERE n.id IN [1, NULL, 3]  // NULL is never equal to anything

// Filter NULLs in application code
```

## Relationship Pattern Errors

### Forgetting Direction

```cypher
// WRONG - Creates both directions
MATCH (a)-[:FOLLOWS]-(b)  // Undirected!

// CORRECT - Specify direction
MATCH (a)-[:FOLLOWS]->(b)  // a follows b
MATCH (a)<-[:FOLLOWS]-(b)  // b follows a
```

### Variable-Length Without Bounds

```cypher
// DANGEROUS - Potentially explosive
MATCH (a)-[*]->(b)  // Any length path!

// SAFE - Set bounds
MATCH (a)-[*1..3]->(b)  // 1 to 3 hops max
```

### Creating Duplicate Relationships

```cypher
// May create duplicates
CREATE (a)-[:KNOWS]->(b)

// Idempotent
MERGE (a)-[:KNOWS]->(b)
```

## Performance Mistakes

### Cartesian Products

```cypher
// WRONG - Cartesian product
MATCH (a:Person), (b:Product)
WHERE a.id = $personId AND b.id = $productId
CREATE (a)-[:BOUGHT]->(b)

// CORRECT - Single pattern or sequential
MATCH (a:Person {id: $personId})
MATCH (b:Product {id: $productId})
CREATE (a)-[:BOUGHT]->(b)
```

### Late Filtering

```cypher
// SLOW - Filters after collecting everything
MATCH (e:Event)
WITH e
WHERE e.kind = 1  // Should be in MATCH or right after

// FAST - Filter early
MATCH (e:Event)
WHERE e.kind = 1
```

### Missing LIMIT with ORDER BY

```cypher
// SLOW - Sorts all results
MATCH (e:Event)
RETURN e
ORDER BY e.created_at DESC

// FAST - Limits result set
MATCH (e:Event)
RETURN e
ORDER BY e.created_at DESC
LIMIT 100
```

### Unparameterized Queries

```cypher
// WRONG - No query plan caching, injection risk
MATCH (e:Event {id: '" + eventId + "'})

// CORRECT - Use parameters
MATCH (e:Event {id: $eventId})
```

## String Comparison Errors

### Case Sensitivity

```cypher
// Cypher strings are case-sensitive
WHERE n.name = 'alice'  // Won't match 'Alice'

// Use toLower/toUpper for case-insensitive
WHERE toLower(n.name) = toLower($name)

// Or use regex with (?i)
WHERE n.name =~ '(?i)alice'
```

### LIKE vs CONTAINS

```cypher
// There's no LIKE in Cypher
WHERE n.name LIKE '%alice%'  // ERROR!

// Use CONTAINS, STARTS WITH, ENDS WITH
WHERE n.name CONTAINS 'alice'
WHERE n.name STARTS WITH 'ali'
WHERE n.name ENDS WITH 'ice'

// Or regex for complex patterns
WHERE n.name =~ '.*ali.*ce.*'
```

## Index Mistakes

### Constraint vs Index

```cypher
// Constraint (also creates index, enforces uniqueness)
CREATE CONSTRAINT foo IF NOT EXISTS FOR (n:Node) REQUIRE n.id IS UNIQUE

// Index only (no uniqueness enforcement)
CREATE INDEX bar IF NOT EXISTS FOR (n:Node) ON (n.id)
```

### Index Not Used

```cypher
// Index on n.id won't help here
WHERE toLower(n.id) = $id  // Function applied to indexed property!

// Store lowercase if needed, or create computed property
```

### Wrong Composite Index Order

```cypher
// Index on (kind, created_at) won't help query by created_at alone
MATCH (e:Event) WHERE e.created_at > $since  // Index not used

// Either create single-property index or query by kind too
CREATE INDEX event_created_at FOR (e:Event) ON (e.created_at)
```

## Transaction Errors

### Read After Write in Same Transaction

```cypher
// In Neo4j, reads in a transaction see the writes
// But be careful with external processes
CREATE (n:Node {id: 'new'})
WITH n
MATCH (m:Node {id: 'new'})  // Will find 'n'
```

### Locks and Deadlocks

```cypher
// MERGE takes locks; avoid complex patterns that might deadlock
// Bad: two MERGEs on same labels in different order
Session 1: MERGE (a:Person {id: 1}) MERGE (b:Person {id: 2})
Session 2: MERGE (b:Person {id: 2}) MERGE (a:Person {id: 1})  // Potential deadlock

// Good: consistent ordering
Session 1: MERGE (a:Person {id: 1}) MERGE (b:Person {id: 2})
Session 2: MERGE (a:Person {id: 1}) MERGE (b:Person {id: 2})
```

## Type Coercion Issues

### Integer vs String

```cypher
// Types must match
WHERE n.id = 123    // Won't match if n.id is "123"
WHERE n.id = '123'  // Won't match if n.id is 123

// Use appropriate parameter types from Go
params["id"] = int64(123)   // For integer
params["id"] = "123"        // For string
```

### Boolean Handling

```cypher
// Neo4j booleans vs strings
WHERE n.active = true     // Boolean
WHERE n.active = 'true'   // String - different!
```

## Delete Errors

### Delete Node With Relationships

```cypher
// ERROR - Node still has relationships
MATCH (n:Person {id: $id})
DELETE n

// CORRECT - Delete relationships first
MATCH (n:Person {id: $id})
DETACH DELETE n
```

### Optional Match and Delete

```cypher
// WRONG - DELETE NULL causes no error but also doesn't help
OPTIONAL MATCH (n:Node {id: $id})
DELETE n  // If n is NULL, nothing happens silently

// Better - Check existence first or handle in application
MATCH (n:Node {id: $id})
DELETE n
```

## Debugging Tips

1. **Use EXPLAIN** to see query plan without executing
2. **Use PROFILE** to see actual execution metrics
3. **Break complex queries** into smaller parts to isolate issues
4. **Check parameter types** - mismatched types are a common issue
5. **Verify indexes exist** with `SHOW INDEXES`
6. **Check constraints** with `SHOW CONSTRAINTS`
