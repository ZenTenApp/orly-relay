// TinyJS Runtime — Type System
// Runtime type information, interface satisfaction, type assertions.

// Type registry: maps type ID strings to type descriptors.
const typeRegistry = new Map();

// Register a type. Called by generated code at init time.
export function registerType(id, descriptor) {
  typeRegistry.set(id, descriptor);
}

// Type descriptor shape:
// {
//   id: string,           // unique type identifier (package path + name)
//   kind: string,         // 'struct', 'interface', 'basic', 'pointer', 'slice', 'map', 'chan', 'func'
//   methods: Map<string, Function>,  // method name -> implementation
//   fields: [{name, type, tag, embedded}],  // for structs
//   elem: string,         // for pointer/slice/chan: element type ID
//   key: string,          // for maps: key type ID
//   size: number,         // byte size (for basic types)
//   zero: any,            // zero value factory
// }

export function getType(id) {
  return typeRegistry.get(id);
}

// Interface value: { $type: typeId, $value: concreteValue }
export function makeInterface(typeId, value) {
  return { $type: typeId, $value: value };
}

// Type assertion: iface.(T)
// Returns value if type matches, panics otherwise.
export function typeAssert(iface, targetTypeId) {
  if (iface === null || iface === undefined) {
    throw new Error(`interface conversion: interface is nil, not ${targetTypeId}`);
  }
  if (iface.$type === targetTypeId) {
    return iface.$value;
  }
  throw new Error(`interface conversion: interface is ${iface.$type}, not ${targetTypeId}`);
}

// Comma-ok type assertion: v, ok := iface.(T)
export function typeAssertOk(iface, targetTypeId) {
  if (iface === null || iface === undefined) {
    return [zeroForType(targetTypeId), false];
  }
  if (iface.$type === targetTypeId) {
    return [iface.$value, true];
  }
  return [zeroForType(targetTypeId), false];
}

// Interface type assertion: iface.(SomeInterface)
// Checks if the concrete type implements the interface methods.
export function interfaceAssert(iface, interfaceTypeId) {
  if (iface === null || iface === undefined) {
    throw new Error(`interface conversion: interface is nil, not ${interfaceTypeId}`);
  }

  const ifaceType = typeRegistry.get(interfaceTypeId);
  if (!ifaceType || ifaceType.kind !== 'interface') {
    throw new Error(`not an interface type: ${interfaceTypeId}`);
  }

  const concreteType = typeRegistry.get(iface.$type);
  if (!concreteType) {
    throw new Error(`unknown type: ${iface.$type}`);
  }

  // Check all interface methods are satisfied.
  for (const [name] of ifaceType.methods) {
    if (!concreteType.methods || !concreteType.methods.has(name)) {
      throw new Error(
        `interface conversion: ${iface.$type} does not implement ${interfaceTypeId} (missing method ${name})`
      );
    }
  }

  return iface;
}

// Interface method call dispatch.
export function methodCall(iface, methodName, args) {
  if (iface === null || iface === undefined) {
    throw new Error('runtime error: invalid memory address or nil pointer dereference');
  }

  const concreteType = typeRegistry.get(iface.$type);
  if (!concreteType || !concreteType.methods || !concreteType.methods.has(methodName)) {
    throw new Error(`method not found: ${iface.$type}.${methodName}`);
  }

  const method = concreteType.methods.get(methodName);
  return method(iface.$value, ...args);
}

// Type switch helper.
export function typeSwitch(iface) {
  if (iface === null || iface === undefined) {
    return { type: null, value: null };
  }
  return { type: iface.$type, value: iface.$value };
}

// Zero value for a type.
function zeroForType(typeId) {
  const desc = typeRegistry.get(typeId);
  if (desc && desc.zero) return desc.zero();

  // Fallback for basic types.
  if (typeId === 'bool') return false;
  if (typeId === 'string') return '';
  if (typeId.startsWith('int') || typeId.startsWith('uint') ||
      typeId.startsWith('float') || typeId === 'uintptr' || typeId === 'byte' || typeId === 'rune') {
    return 0;
  }
  return null;
}

// Comparable check (for map keys).
export function comparable(a, b) {
  if (a === b) return true;
  if (a === null || b === null) return false;
  if (typeof a !== typeof b) return false;
  if (typeof a === 'object') {
    // Struct comparison: compare all fields.
    const keysA = Object.keys(a).filter(k => !k.startsWith('$'));
    const keysB = Object.keys(b).filter(k => !k.startsWith('$'));
    if (keysA.length !== keysB.length) return false;
    return keysA.every(k => comparable(a[k], b[k]));
  }
  return false;
}
