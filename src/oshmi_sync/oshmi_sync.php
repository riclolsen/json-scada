<?PHP

header('Content-Type: application/json; charset=utf-8');

// This path should point to Composer's autoloader
require 'vendor/autoload.php';

ini_set('display_errors', 1);
ini_set('display_startup_errors', 1);
error_reporting(E_ALL);

try
{

$strconf = file_get_contents('../../conf/json-scada.json');
$conf = json_decode($strconf, true);

// $conf['mongoConnectionString']='mongodb://localhost:27017/json_scada?replicaSet=rs1&readPreference=primary&ssl=false';

$client = new MongoDB\Client($conf['mongoConnectionString']);
$dbs = $client->listDatabases(); // to check connection
$collection = $client->json_scada->realtimeData;

// Apaga todas as anotações 
$result = $collection->updateMany([], 
['$set' => ['notes' => ''] ]
);
print_r($result);
$result = $collection->updateMany([], 
['$set' => ['annotation' => ''] ]
);
print_r($result);

//$result = $collection->find( [ 'group1' => 'KAW2' ] );
//foreach ($result as $entry) {
//    echo $entry['_id'], ': ', $entry['tag'], "\n";
//}

// busca e sincroniza anotações documentais encontradas abertas

$db = new PDO( 'sqlite:c:/oshmi/db/notes.sl3','','', [
  PDO::ATTR_TIMEOUT => 5,
  PDO::ATTR_ERRMODE => PDO::ERRMODE_EXCEPTION
  ] );
$db->exec ( "PRAGMA synchronous=NORMAL" );
$db->exec ( "PRAGMA journal_mode=WAL" );
$db->exec ( "PRAGMA locking_mode=NORMAL" );
$db->exec ( "PRAGMA cache_size=5000" );
$db->exec ( "PRAGMA temp_store=MEMORY" );

$qry = "select POINTNUM as NPONTO, content from notes where erased=0";
$ret = $db->query($qry);

// echo json_encode($ret);
$o = $ret->fetchAll(PDO::FETCH_ASSOC);
array_walk_recursive($o, 'walkrec');
print_r($o);
foreach ($o as $row)
{
  print_r($row['NPONTO'] . " " . $row['CONTENT']);
  $result = $collection->updateOne(['_id' => floatval($row['NPONTO'])], 
                                   ['$set' => ['notes' => $row['CONTENT']] ]
                                  );
  print_r($result);
}

// busca e sincroniza anotações blocantes, inibidos limites e histerese 

$db = new PDO( 'sqlite:c:/oshmi/db/dumpdb.sl3','','', [
  PDO::ATTR_TIMEOUT => 5,
  PDO::ATTR_ERRMODE => PDO::ERRMODE_EXCEPTION
  ] );
$db->exec ( "PRAGMA synchronous=NORMAL" );
$db->exec ( "PRAGMA journal_mode=WAL" );
$db->exec ( "PRAGMA locking_mode=NORMAL" );
$db->exec ( "PRAGMA cache_size=5000" );
$db->exec ( "PRAGMA temp_store=MEMORY" );

$qry = "select NPONTO, ANOTACAO, ALRIN, ID, LIMS, LIMI, HISTER from dumpdb"; // where ANOTACAO != '' or ALRIN!=0 or LIMS<99999 or LIMI>-99999
$ret = $db->query($qry);

// echo json_encode($ret);
$o = $ret->fetchAll(PDO::FETCH_ASSOC);
array_walk_recursive($o, 'walkrec');
// print_r($o);
foreach ($o as $row)
{
  print_r($row);
  
  $lims = floatval($row['LIMS']);
  if ($lims >= 999999)
    $lims="Infinity";
  $limi = floatval($row['LIMI']);
  if ($limi <= -999999)
    $limi="-Infinity";
  
  $result = $collection->updateOne(['_id' => floatval($row['NPONTO'])], 
                                   ['$set' => 
                                     [
                                       'annotation' => $row['ANOTACAO'],
                                       'alarmDisabled' => $row['ALRIN']?true:false,
                                       'hiLimit' => $lims,
                                       'loLimit' => $limi,
                                       'hysteresis' => $row['HISTER'],
                                     ]
                                   ]
                                  );
 print_r($result);
}


// echo json_encode($o, JSON_PRETTY_PRINT);
}
catch(PDOException $Exception)
{
  echo $Exception->getMessage( );
}

// converte para UTF-8 e faz outras conversões de tipo (para json_encode)
function walkrec(&$item, $key)
  {
  if (!mb_check_encoding($item, "UTF-8"))
    $item = mb_convert_encoding($item, "UTF-8", "ISO-8859-1"); // converte para UTF-8
  }
?>
