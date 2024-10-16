
<?php

//var_dump ($_GET);
/*
foreach ($_GET as $key => $value) {
    echo $key . ' => ' . $value . '<br />';
}
*/


$roughHTTPPOST = file_get_contents("php://input"); 
parse_str($roughHTTPPOST);

$handle = opendir($folder);
while (false !== ($entry = readdir($handle))) {
  if ($entry != "." && $entry != "..") {
    $files[] = $entry;
  }
}
closedir($handle);

echo json_encode($files);

?>
