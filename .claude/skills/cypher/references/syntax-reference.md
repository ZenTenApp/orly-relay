# Cypher Syntax Reference

Complete syntax reference for Neo4j Cypher query language.

## Clause Reference

### Reading Clauses

#### MATCH

Finds patterns in the graph.

```cypher
// Basic node match
MATCH (n:Label)

// Match with properties
MATCH (n:Label {key: value})

// Match relationships
MATCH (a)-[r:RELATES_TO]->(b)

// Match path
MATCH path = (a)-[*1..3]->(b)
```

#### OPTIONAL MATCH

Like MATCH but returns NULL for non-matches (LEFT OUTER JOIN).

```cypher
MATCH (a:Person)
OPTIONAL MATCH (a)-[:KNOWS]->(b:Person)
RETURN a.name, b.name  // b.name may be NULL
```

#### WHERE

Filters results.

```cypher
// Comparison operators
WHERE n.age > 21
WHERE n.age >= 21
WHERE n.age < 65
WHERE n.age <= 65
WHERE n.name = 'Alice'
WHERE n.name <> 'Bob'

// Boolean operators
WHERE n.age > 21 AND n.active = true
WHERE n.age < 18 OR n.age > 65
WHERE NOT n.deleted

// NULL checks
WHERE n.email IS NULL
WHERE n.email IS NOT NULL

// Pattern predicates
WHERE (n)-[:KNOWS]->(:Person)
WHERE NOT (n)-[:BLOCKED]->()
WHERE exists((n)-[:FOLLOWS]->())

// String predicates
WHERE n.name STARTS WITH 'A'
WHERE n.name ENDS WITH 'son'
WHERE n.name CONTAINS 'li'
WHERE n.name =~ '(?i)alice.*'  // Case-insensitive regex

// List predicates
WHERE n.status IN ['active', 'pending']
WHERE any(x IN n.tags WHERE x = 'important')
WHERE all(x IN n.scores WHERE x > 50)
WHERE none(x IN n.errors WHERE x IS NOT NULL)
WHERE single(x IN n.items WHERE x.primary = true)
```

### Writing Clauses

#### CREATE

Creates nodes and relationships.

```cypher
// Create node
CREATE (n:Label {key: value})

// Create multiple nodes
CREATE (a:Person {name: 'Alice'}), (b:Person {name: 'Bob'})

// Create relationship
CREATE (a)-[r:KNOWS {since: 2020}]->(b)

// Create path
CREATE p = (a)-[:KNOWS]->(b)-[:KNOWS]->(c)
```

#### MERGE

Find or create pattern. **Critical for idempotency**.

```cypher
// MERGE node
MERGE (n:Label {key: $uniqueKey})

// MERGE with ON CREATE / ON MATCH
MERGE (n:Person {email: $email})
ON CREATE SET n.created = timestamp(), n.name = $name
ON MATCH SET n.accessed = timestamp()

// MERGE relationship (both nodes must exist or be in scope)
MERGE (a)-[r:KNOWS]->(b)
ON CREATE SET r.since = date()
```

**MERGE Gotcha**: MERGE on a pattern locks the entire pattern. For relationships, MERGE each node first:

```cypher
// CORRECT
MERGE (a:Person {id: $id1})
MERGE (b:Person {id: $id2})
MERGE (a)-[:KNOWS]->(b)

// RISKY - may create duplicate nodes
MERGE (a:Person {id: $id1})-[:KNOWS]->(b:Person {id: $id2})
```

#### SET

Updates properties.

```cypher
// Set single property
SET n.name = 'Alice'

// Set multiple properties
SET n.name = 'Alice', n.age = 30

// Set from map (replaces all properties)
SET n = {name: 'Alice', age: 30}

// Set from map (adds/updates, keeps existing)
SET n += {name: 'Alice'}

// Set label
SET n:NewLabel

// Remove property
SET n.obsolete = null
```

#### DELETE / DETACH DELETE

Removes nodes and relationships.

```cypher
// Delete relationship
MATCH (a)-[r:KNOWS]->(b)
DELETE r

// Delete node (must have no relationships)
MATCH (n:Orphan)
DELETE n

// Delete node and all relationships
MATCH (n:Person {name: 'Bob'})
DETACH DELETE n
```

#### REMOVE

Removes properties and labels.

```cypher
// Remove property
REMOVE n.temporary

// Remove label
REMOVE n:OldLabel
```

### Projection Clauses

#### RETURN

Specifies output.

```cypher
// Return nodes
RETURN n

// Return properties
RETURN n.name, n.age

// Return with alias
RETURN n.name AS name, n.age AS age

// Return all
RETURN *

// Return distinct
RETURN DISTINCT n.category

// Return expression
RETURN n.price * n.quantity AS total
```

#### WITH

Passes results between query parts. **Critical for multi-part queries**.

```cypher
// Filter and pass
MATCH (n:Person)
WITH n WHERE n.age > 21
RETURN n

// Aggregate and continue
MATCH (n:Person)-[:BOUGHT]->(p:Product)
WITH n, count(p) AS purchases
WHERE purchases > 5
RETURN n.name, purchases

// Order and limit mid-query
MATCH (n:Person)
WITH n ORDER BY n.age DESC LIMIT 10
MATCH (n)-[:LIVES_IN]->(c:City)
RETURN n.name, c.name
```

**WITH resets scope**: Variables not listed in WITH are no longer available.

#### ORDER BY

Sorts results.

```cypher
ORDER BY n.name                    // Ascending (default)
ORDER BY n.name ASC               // Explicit ascending
ORDER BY n.name DESC              // Descending
ORDER BY n.lastName, n.firstName  // Multiple fields
ORDER BY n.priority DESC, n.name  // Mixed
```

#### SKIP and LIMIT

Pagination.

```cypher
// Skip first 10
SKIP 10

// Return only 20
LIMIT 20

// Pagination
ORDER BY n.created_at DESC
SKIP $offset LIMIT $pageSize
```

### Sub-queries

#### CALL (Subquery)

Execute subquery for each row.

```cypher
MATCH (p:Person)
CALL {
  WITH p
  MATCH (p)-[:BOUGHT]->(prod:Product)
  RETURN count(prod) AS purchaseCount
}
RETURN p.name, purchaseCount
```

#### UNION

Combine results from multiple queries.

```cypher
MATCH (n:Person) RETURN n.name AS name
UNION
MATCH (n:Company) RETURN n.name AS name

// UNION ALL keeps duplicates
MATCH (n:Person) RETURN n.name AS name
UNION ALL
MATCH (n:Company) RETURN n.name AS name
```

### Control Flow

#### FOREACH

Iterate over list, execute updates.

```cypher
// Set property on path nodes
MATCH path = (a)-[*]->(b)
FOREACH (n IN nodes(path) | SET n.visited = true)

// Conditional operation (common pattern)
OPTIONAL MATCH (target:Node {id: $id})
FOREACH (_ IN CASE WHEN target IS NOT NULL THEN [1] ELSE [] END |
  CREATE (source)-[:LINKS_TO]->(target)
)
```

#### CASE

Conditional expressions.

```cypher
// Simple CASE
RETURN CASE n.status
  WHEN 'active' THEN 'A'
  WHEN 'pending' THEN 'P'
  ELSE 'X'
END AS code

// Generic CASE
RETURN CASE
  WHEN n.age < 18 THEN 'minor'
  WHEN n.age < 65 THEN 'adult'
  ELSE 'senior'
END AS category
```

## Operators

### Comparison

| Operator | Description |
|----------|-------------|
| `=` | Equal |
| `<>` | Not equal |
| `<` | Less than |
| `>` | Greater than |
| `<=` | Less than or equal |
| `>=` | Greater than or equal |
| `IS NULL` | Is null |
| `IS NOT NULL` | Is not null |

### Boolean

| Operator | Description |
|----------|-------------|
| `AND` | Logical AND |
| `OR` | Logical OR |
| `NOT` | Logical NOT |
| `XOR` | Exclusive OR |

### String

| Operator | Description |
|----------|-------------|
| `STARTS WITH` | Prefix match |
| `ENDS WITH` | Suffix match |
| `CONTAINS` | Substring match |
| `=~` | Regex match |

### List

| Operator | Description |
|----------|-------------|
| `IN` | List membership |
| `+` | List concatenation |

### Mathematical

| Operator | Description |
|----------|-------------|
| `+` | Addition |
| `-` | Subtraction |
| `*` | Multiplication |
| `/` | Division |
| `%` | Modulo |
| `^` | Exponentiation |

## Functions

### Aggregation

```cypher
count(*)              // Count rows
count(n)              // Count non-null
count(DISTINCT n)     // Count unique
sum(n.value)          // Sum
avg(n.value)          // Average
min(n.value)          // Minimum
max(n.value)          // Maximum
collect(n)            // Collect to list
collect(DISTINCT n)   // Collect unique
stDev(n.value)        // Standard deviation
percentileCont(n.value, 0.5)  // Median
```

### Scalar

```cypher
// Type functions
id(n)                 // Internal node ID (deprecated, use elementId)
elementId(n)          // Element ID string
labels(n)             // Node labels
type(r)               // Relationship type
properties(n)         // Property map

// Math
abs(x)
ceil(x)
floor(x)
round(x)
sign(x)
sqrt(x)
rand()                // Random 0-1

// String
size(str)             // String length
toLower(str)
toUpper(str)
trim(str)
ltrim(str)
rtrim(str)
replace(str, from, to)
substring(str, start, len)
left(str, len)
right(str, len)
split(str, delimiter)
reverse(str)
toString(val)

// Null handling
coalesce(val1, val2, ...)  // First non-null
nullIf(val1, val2)         // NULL if equal

// Type conversion
toInteger(val)
toFloat(val)
toBoolean(val)
toString(val)
```

### List Functions

```cypher
size(list)            // List length
head(list)            // First element
tail(list)            // All but first
last(list)            // Last element
range(start, end)     // Create range [start..end]
range(start, end, step)
reverse(list)
keys(map)             // Map keys as list
values(map)           // Map values as list

// List predicates
any(x IN list WHERE predicate)
all(x IN list WHERE predicate)
none(x IN list WHERE predicate)
single(x IN list WHERE predicate)

// List manipulation
[x IN list WHERE predicate]           // Filter
[x IN list | expression]              // Map
[x IN list WHERE pred | expr]         // Filter and map
reduce(s = initial, x IN list | s + x) // Reduce
```

### Path Functions

```cypher
nodes(path)           // Nodes in path
relationships(path)   // Relationships in path
length(path)          // Number of relationships
shortestPath((a)-[*]-(b))
allShortestPaths((a)-[*]-(b))
```

### Temporal Functions

```cypher
timestamp()           // Current Unix timestamp (ms)
datetime()            // Current datetime
date()                // Current date
time()                // Current time
duration({days: 1, hours: 12})

// Components
datetime().year
datetime().month
datetime().day
datetime().hour

// Parsing
date('2024-01-15')
datetime('2024-01-15T10:30:00Z')
```

### Spatial Functions

```cypher
point({x: 1, y: 2})
point({latitude: 37.5, longitude: -122.4})
distance(point1, point2)
```

## Comments

```cypher
// Single line comment

/* Multi-line
   comment */
```

## Transaction Control

```cypher
// In procedures/transactions
:begin
:commit
:rollback
```

## Parameter Syntax

```cypher
// Parameter reference
$paramName

// In properties
{key: $value}

// In WHERE
WHERE n.id = $id

// In expressions
RETURN $multiplier * n.value
```
