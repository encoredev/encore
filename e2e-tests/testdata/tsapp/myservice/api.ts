import { api } from "encore.dev/api";

export const hello = api(
  { expose: true, method: "GET", path: "/hello/:name" },
  async ({ name }: { name: string }): Promise<{ message: string }> => {
    return { message: `Hello ${name}` };
  }
);
