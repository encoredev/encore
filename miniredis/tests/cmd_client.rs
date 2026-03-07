mod helpers;

#[tokio::test]
async fn test_client_setname_getname() {
    let (_m, mut c) = helpers::start().await;

    // Set the client name
    must_ok!(c, "CLIENT", "SETNAME", "miniredis-tests");

    // Get the client name
    must_str!(c, "CLIENT", "GETNAME"; "miniredis-tests");
}

#[tokio::test]
async fn test_client_getname_without_setname() {
    let (_m, mut c) = helpers::start().await;

    // Get the client name without setting it first → nil
    must_nil!(c, "CLIENT", "GETNAME");
}

#[tokio::test]
async fn test_client_setname_empty() {
    let (_m, mut c) = helpers::start().await;

    // Set then clear the client name
    must_ok!(c, "CLIENT", "SETNAME", "test");
    must_str!(c, "CLIENT", "GETNAME"; "test");

    must_ok!(c, "CLIENT", "SETNAME", "");
    must_nil!(c, "CLIENT", "GETNAME");
}

#[tokio::test]
async fn test_client_errors() {
    let (_m, mut c) = helpers::start().await;

    must_fail!(c, "CLIENT"; "wrong number of arguments");
    must_fail!(c, "CLIENT", "NOSUCHSUB"; "unknown subcommand");
}
