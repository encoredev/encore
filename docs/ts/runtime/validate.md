---
title: encore.dev/validate
lang: ts
toc: true
---

# encore.dev/validate

## Type Aliases

### EndsWith

```ts
type EndsWith<S> = {
  [___validate]?: {
     endsWith: S;
  };
};
```

Defined in: [validate/mod.ts:39](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L39)

#### Type Parameters

##### S

`S` *extends* `string`

#### Properties

##### \[\_\_\_validate\]?

```ts
optional [___validate]?: {
  endsWith: S;
};
```

Defined in: [validate/mod.ts:40](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L40)

###### endsWith

```ts
endsWith: S;
```

***

### IsEmail

```ts
type IsEmail = {
  [___validate]?: {
     isEmail: true;
  };
};
```

Defined in: [validate/mod.ts:45](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L45)

#### Properties

##### \[\_\_\_validate\]?

```ts
optional [___validate]?: {
  isEmail: true;
};
```

Defined in: [validate/mod.ts:46](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L46)

###### isEmail

```ts
isEmail: true;
```

***

### IsURL

```ts
type IsURL = {
  [___validate]?: {
     isURL: true;
  };
};
```

Defined in: [validate/mod.ts:51](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L51)

#### Properties

##### \[\_\_\_validate\]?

```ts
optional [___validate]?: {
  isURL: true;
};
```

Defined in: [validate/mod.ts:52](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L52)

###### isURL

```ts
isURL: true;
```

***

### MatchesRegexp

```ts
type MatchesRegexp<S> = {
  [___validate]?: {
     matchesRegexp: S;
  };
};
```

Defined in: [validate/mod.ts:27](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L27)

#### Type Parameters

##### S

`S` *extends* `string`

#### Properties

##### \[\_\_\_validate\]?

```ts
optional [___validate]?: {
  matchesRegexp: S;
};
```

Defined in: [validate/mod.ts:28](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L28)

###### matchesRegexp

```ts
matchesRegexp: S;
```

***

### Max

```ts
type Max<N> = {
  [___validate]?: {
     maxValue: N;
  };
};
```

Defined in: [validate/mod.ts:9](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L9)

#### Type Parameters

##### N

`N` *extends* `number`

#### Properties

##### \[\_\_\_validate\]?

```ts
optional [___validate]?: {
  maxValue: N;
};
```

Defined in: [validate/mod.ts:10](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L10)

###### maxValue

```ts
maxValue: N;
```

***

### MaxLen

```ts
type MaxLen<N> = {
  [___validate]?: {
     maxLen: N;
  };
};
```

Defined in: [validate/mod.ts:21](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L21)

#### Type Parameters

##### N

`N` *extends* `number`

#### Properties

##### \[\_\_\_validate\]?

```ts
optional [___validate]?: {
  maxLen: N;
};
```

Defined in: [validate/mod.ts:22](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L22)

###### maxLen

```ts
maxLen: N;
```

***

### Min

```ts
type Min<N> = {
  [___validate]?: {
     minValue: N;
  };
};
```

Defined in: [validate/mod.ts:3](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L3)

#### Type Parameters

##### N

`N` *extends* `number`

#### Properties

##### \[\_\_\_validate\]?

```ts
optional [___validate]?: {
  minValue: N;
};
```

Defined in: [validate/mod.ts:4](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L4)

###### minValue

```ts
minValue: N;
```

***

### MinLen

```ts
type MinLen<N> = {
  [___validate]?: {
     minLen: N;
  };
};
```

Defined in: [validate/mod.ts:15](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L15)

#### Type Parameters

##### N

`N` *extends* `number`

#### Properties

##### \[\_\_\_validate\]?

```ts
optional [___validate]?: {
  minLen: N;
};
```

Defined in: [validate/mod.ts:16](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L16)

###### minLen

```ts
minLen: N;
```

***

### StartsWith

```ts
type StartsWith<S> = {
  [___validate]?: {
     startsWith: S;
  };
};
```

Defined in: [validate/mod.ts:33](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L33)

#### Type Parameters

##### S

`S` *extends* `string`

#### Properties

##### \[\_\_\_validate\]?

```ts
optional [___validate]?: {
  startsWith: S;
};
```

Defined in: [validate/mod.ts:34](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L34)

###### startsWith

```ts
startsWith: S;
```
