declare const __validate: unique symbol;

export type Between<Min extends number, Max extends number> = {
  [__validate]: {
    minValue: Min;
    maxValue: Max;
  };
};

export type Min<N extends number> = {
  [__validate]: {
    minValue: N;
  };
};

export type Max<N extends number> = {
  [__validate]: {
    maxValue: N;
  };
};

export type MinLen<N extends number> = {
  [__validate]?: {
    minLen: N;
  };
};

export type MaxLen<N extends number> = {
  [__validate]?: {
    maxLen: N;
  };
};
