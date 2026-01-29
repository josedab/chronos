---
sidebar_position: 3
title: WASM Plugins
description: Extend Chronos with WebAssembly plugins
---

# WebAssembly Plugins

Custom job logic via WASM modules.

## Use Cases

- Custom validation logic
- Data transformation
- Pre/post execution hooks

## Example

```rust
#[no_mangle]
pub fn process(input: &[u8]) -> Vec<u8> {
    // Custom processing logic
    input.to_vec()
}
```
