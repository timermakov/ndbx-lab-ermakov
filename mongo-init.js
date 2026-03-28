const dbName = process.env.MONGODB_DATABASE;
const username = process.env.MONGODB_USER;
const password = process.env.MONGODB_PASSWORD;

if (!dbName || !username || !password) {
  throw new Error("mongodb init: required env vars are missing");
}

const appDb = db.getSiblingDB(dbName);
appDb.createUser({
  user: username,
  pwd: password,
  roles: [
    { role: "readWrite", db: dbName },
    { role: "dbOwner", db: dbName },
  ],
});
