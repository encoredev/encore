module.exports = {
  testEnvironment: "jsdom",
  moduleNameMapper: {
    "^~c/(.*)$": "<rootDir>/src/components/$1",
    "^~lib/(.*)$": "<rootDir>/src/lib/$1",
    "^~mod/(.*)$": "<rootDir>/src/mod/$1",
    "^~p/(.*)$": "<rootDir>/src/pages/$1",
  },
  setupFilesAfterEnv: ["<rootDir>/jest.setup.js"],
};
