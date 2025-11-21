export abstract class TopicPerms {
  private topicPerms(): void {}
}

export abstract class Publisher<Msg extends object> extends TopicPerms {
  abstract publish(msg: Msg): Promise<string>;
}
