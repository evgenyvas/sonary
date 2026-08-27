<?php
require_once('plugins/login-password-less.php');

/**
 * Set an allowed dummy password for the web interface
 * The default hash below matches the password "admin"
 */
return new AdminerLoginPasswordLess(
    $password_hash = password_hash("admin", PASSWORD_DEFAULT)
);
