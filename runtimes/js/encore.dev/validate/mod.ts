const validate = Symbol("validate");

export interface Validator {
  [validate]?: {};
}

export type Validate<T, V extends Validator> = T & V;

export type Between<Min extends number, Max extends number> = {
  [validate]?: {
    minValue: Min;
    maxValue: Max;
  };
};

export type Min<N extends number> = {
  [validate]?: {
    minValue: N;
  };
};

export type Max<N extends number> = {
  [validate]?: {
    maxValue: N;
  };
};

export type MinLen<N extends number> = {
  [validate]?: {
    minLen: N;
  };
};

export type MaxLen<N extends number> = {
  [validate]?: {
    maxLen: N;
  };
};
