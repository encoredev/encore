export { Topic } from "./topic";
export type { TopicConfig, DeliveryGuarantee } from "./topic";

export { Subscription } from "./subscription";
export type { SubscriptionConfig } from "./subscription";

/**
 * Attribute represents a field on a message that should be sent
 * as an attribute in a PubSub message, rather than in the message
 * body.
 *
 * This is useful for ordering messages, or for filtering messages
 * on a subscription - otherwise you should not use this.
 *
 * To create attributes on a message, use the `Attribute` type:
 *   type Message = {
 *     user_id: Attribute<number>;
 *     name: string;
 *   };
 *
 *   const msg: Message = {
 *     user_id: 123,
 *     name:    "John Doe",
 *   };
 *
 * The union of brandedAttribute is simply used to help the TypeScript compiler
 * understand that the type is an attribute and allow the AttributesOf type
 * to extract the keys of said type.
 */
export type Attribute<T extends string | number | boolean> =
  | T
  | brandedAttribute<T>;

/**
 * AttributesOf is a helper type to extract all keys from an object
 * who's type is an Attribute type.
 *
 * For example:
 *    type Message = {
 *        user_id: Attribute<number>;
 *        name: string;
 *        age: Attribute<number>;
 *    };
 *
 *    type MessageAttributes = AttributesOf<Message>; // "user_id" | "age"
 */
export type AttributesOf<T extends object> = keyof {
  [Key in keyof // for (const Key in T)
  T as Extract<T[Key], allBrandedTypes> extends never //  if (typeof T[Key] !== oneof(allBrandedTypes))
  ? never // drop the key
  : Key]: never; // else keep the key
};

/**
 * supportedAttributeTypes is a union of all primitive types that are supported as attributes
 */
type supportedAttributeTypes = string | number | boolean;

/**
 * brandedAttribute is a helper type to brand a type as an attribute
 * which is distinct from the base type. It is a compile time only
 * type and has no runtime representation.
 */
type brandedAttribute<T> = T & { readonly __attributeBrand: unique symbol };

/**
 * allBrandedTypes is a helper type to create a union of all branded supported attribute types
 *
 * The result of this is: brandedAttribute<string> | brandedAttribute<number> | brandedAttribute<boolean>
 */
type allBrandedTypes<Union = supportedAttributeTypes> =
  Union extends supportedAttributeTypes ? brandedAttribute<Union> : never;
