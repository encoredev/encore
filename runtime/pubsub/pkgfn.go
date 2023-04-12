//go:build encore_app

package pubsub

// NewTopic is used to declare a Topic. Encore will use static
// analysis to identify Topics and automatically provision them
// for you.
//
// A call to NewTopic can only be made when declaring a package level variable. Any
// calls to this function made outside a package level variable declaration will result
// in a compiler error.
//
// The topic name must be unique within an Encore application. Topic names must be defined
// in kebab-case (lowercase alphanumerics and hyphen seperated). The topic name must start with a letter
// and end with either a letter or number. It cannot be longer than 63 characters. Once created and deployed never
// change the topic name. When refactoring the topic name must stay the same.
// This allows for messages already on the topic to continue to be received after the refactored
// code is deployed.
//
// Example:
//
//	 import "encore.dev/pubsub"
//
//	 type MyEvent struct {
//	   Foo string
//	 }
//
//	 var MyTopic = pubsub.NewTopic[*MyEvent]("my-topic", pubsub.TopicConfig{
//	   DeliveryGuarantee: pubsub.AtLeastOnce,
//	 })
//
//	//encore:api public
//	func DoFoo(ctx context.Context) error {
//	  msgID, err := MyTopic.Publish(ctx, &MyEvent{Foo: "bar"})
//	  if err != nil { return err }
//	  rlog.Info("foo published", "message_id", msgID)
//	  return nil
//	}
func NewTopic[T any](name string, cfg TopicConfig) *Topic[T] {
	return newTopic[T](Singleton, name, cfg)
}
