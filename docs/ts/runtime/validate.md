---
title: encore.dev/validate
lang: ts
toc: true
---

## Type Aliases

<!-- symbol-start: EndsWith -->
### EndsWith

```ts
type EndsWith<S> = {
  [___validate]?: {
     endsWith: S;
  };
};
```

<!-- source: validate/mod.ts:39 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L39)

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

<!-- source: validate/mod.ts:40 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L40)

###### endsWith

`endsWith: S`

***

<!-- symbol-end -->

<!-- symbol-start: IsEmail -->
### IsEmail

```ts
type IsEmail = {
  [___validate]?: {
     isEmail: true;
  };
};
```

<!-- source: validate/mod.ts:45 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L45)

#### Properties

##### \[\_\_\_validate\]?

```ts
optional [___validate]?: {
  isEmail: true;
};
```

<!-- source: validate/mod.ts:46 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L46)

###### isEmail

`isEmail: true`

***

<!-- symbol-end -->

<!-- symbol-start: IsURL -->
### IsURL

```ts
type IsURL = {
  [___validate]?: {
     isURL: true;
  };
};
```

<!-- source: validate/mod.ts:51 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L51)

#### Properties

##### \[\_\_\_validate\]?

```ts
optional [___validate]?: {
  isURL: true;
};
```

<!-- source: validate/mod.ts:52 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L52)

###### isURL

`isURL: true`

***

<!-- symbol-end -->

<!-- symbol-start: MatchesRegexp -->
### MatchesRegexp

```ts
type MatchesRegexp<S> = {
  [___validate]?: {
     matchesRegexp: S;
  };
};
```

<!-- source: validate/mod.ts:27 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L27)

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

<!-- source: validate/mod.ts:28 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L28)

###### matchesRegexp

`matchesRegexp: S`

***

<!-- symbol-end -->

<!-- symbol-start: Max -->
### Max

```ts
type Max<N> = {
  [___validate]?: {
     maxValue: N;
  };
};
```

<!-- source: validate/mod.ts:9 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L9)

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

<!-- source: validate/mod.ts:10 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L10)

###### maxValue

`maxValue: N`

***

<!-- symbol-end -->

<!-- symbol-start: MaxLen -->
### MaxLen

```ts
type MaxLen<N> = {
  [___validate]?: {
     maxLen: N;
  };
};
```

<!-- source: validate/mod.ts:21 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L21)

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

<!-- source: validate/mod.ts:22 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L22)

###### maxLen

`maxLen: N`

***

<!-- symbol-end -->

<!-- symbol-start: Min -->
### Min

```ts
type Min<N> = {
  [___validate]?: {
     minValue: N;
  };
};
```

<!-- source: validate/mod.ts:3 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L3)

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

<!-- source: validate/mod.ts:4 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L4)

###### minValue

`minValue: N`

***

<!-- symbol-end -->

<!-- symbol-start: MinLen -->
### MinLen

```ts
type MinLen<N> = {
  [___validate]?: {
     minLen: N;
  };
};
```

<!-- source: validate/mod.ts:15 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L15)

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

<!-- source: validate/mod.ts:16 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L16)

###### minLen

`minLen: N`

***

<!-- symbol-end -->

<!-- symbol-start: StartsWith -->
### StartsWith

```ts
type StartsWith<S> = {
  [___validate]?: {
     startsWith: S;
  };
};
```

<!-- source: validate/mod.ts:33 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L33)

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

<!-- source: validate/mod.ts:34 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L34)

###### startsWith

`startsWith: S`


<!-- symbol-end -->