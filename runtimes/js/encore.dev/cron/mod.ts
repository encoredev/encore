/* eslint-disable */

import { DurationString } from "../internal/types/mod";

export class CronJob {
  public readonly name: string;
  public readonly cfg: CronJobConfig;
  constructor(name: string, cfg: CronJobConfig) {
    this.name = name;
    this.cfg = cfg;
  }
}

export type CronJobConfig = {
  endpoint: () => Promise<unknown>;
  title?: string;
} & ({ every: DurationString } | { schedule: string });
