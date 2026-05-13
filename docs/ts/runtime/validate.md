---
title: encore.dev/validate
lang: ts
toc: true
---

## Type Aliases

<!-- symbol-start: EndsWith -->
### EndsWith <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L39" target="_blank" rel="noopener">source</a>

```ts
type EndsWith<S> = {
  [___validate]?: {
     endsWith: S;
  };
};
```

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

###### endsWith

`endsWith: S`

***

<!-- symbol-end -->

<!-- symbol-start: IsEmail -->
### IsEmail <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L45" target="_blank" rel="noopener">source</a>

```ts
type IsEmail = {
  [___validate]?: {
     isEmail: true;
  };
};
```

#### Properties

##### \[\_\_\_validate\]?

```ts
optional [___validate]?: {
  isEmail: true;
};
```

###### isEmail

`isEmail: true`

***

<!-- symbol-end -->

<!-- symbol-start: IsURL -->
### IsURL <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L51" target="_blank" rel="noopener">source</a>

```ts
type IsURL = {
  [___validate]?: {
     isURL: true;
  };
};
```

#### Properties

##### \[\_\_\_validate\]?

```ts
optional [___validate]?: {
  isURL: true;
};
```

###### isURL

`isURL: true`

***

<!-- symbol-end -->

<!-- symbol-start: MatchesRegexp -->
### MatchesRegexp <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L27" target="_blank" rel="noopener">source</a>

```ts
type MatchesRegexp<S> = {
  [___validate]?: {
     matchesRegexp: S;
  };
};
```

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

###### matchesRegexp

`matchesRegexp: S`

***

<!-- symbol-end -->

<!-- symbol-start: Max -->
### Max <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L9" target="_blank" rel="noopener">source</a>

```ts
type Max<N> = {
  [___validate]?: {
     maxValue: N;
  };
};
```

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

###### maxValue

`maxValue: N`

***

<!-- symbol-end -->

<!-- symbol-start: MaxLen -->
### MaxLen <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L21" target="_blank" rel="noopener">source</a>

```ts
type MaxLen<N> = {
  [___validate]?: {
     maxLen: N;
  };
};
```

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

###### maxLen

`maxLen: N`

***

<!-- symbol-end -->

<!-- symbol-start: Min -->
### Min <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L3" target="_blank" rel="noopener">source</a>

```ts
type Min<N> = {
  [___validate]?: {
     minValue: N;
  };
};
```

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

###### minValue

`minValue: N`

***

<!-- symbol-end -->

<!-- symbol-start: MinLen -->
### MinLen <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L15" target="_blank" rel="noopener">source</a>

```ts
type MinLen<N> = {
  [___validate]?: {
     minLen: N;
  };
};
```

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

###### minLen

`minLen: N`

***

<!-- symbol-end -->

<!-- symbol-start: StartsWith -->
### StartsWith <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/validate/mod.ts#L33" target="_blank" rel="noopener">source</a>

```ts
type StartsWith<S> = {
  [___validate]?: {
     startsWith: S;
  };
};
```

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

###### startsWith

`startsWith: S`


<!-- symbol-end -->