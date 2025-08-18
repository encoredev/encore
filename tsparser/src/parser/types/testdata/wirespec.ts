import { HttpStatus } from "encore.dev/api";

export interface ResponseWithStatus {
  data: string;
  status: HttpStatus;
}
