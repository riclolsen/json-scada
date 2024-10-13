
<?php

$roughHTTPPOST = file_get_contents("php://input"); 
parse_str($roughHTTPPOST);
$data = stripslashes($data);


/**
 * JSON beautifier
 * 
 * @param string    The original JSON string
 * @param   string  Return string
 * @param string    Tab string
 * @return string
 */
/**
function pretty_json($data, $ret= "\n", $ind="\t") {

  $beauty_json = '';
  $quote_state = FALSE;
  $level = 0; 

  $json_length = strlen($data);

  for ($i = 0; $i < $json_length; $i++)
  {                               

      $pre = '';
      $suf = '';

      switch ($data[$i])
      {
          case '"':                               
              $quote_state = !$quote_state;                                                           
              break;

          case '[':                                                           
              $level++;               
              break;

          case ']':
              $level--;                   
              $pre = $ret;
              $pre .= str_repeat($ind, $level);       
              break;

          case '{':

              if ($i - 1 >= 0 && $data[$i - 1] != ',')
              {
                  $pre = $ret;
                  $pre .= str_repeat($ind, $level);                       
              }   

              $level++;   
              $suf = $ret;                                                                                                                        
              $suf .= str_repeat($ind, $level);                                                                                                   
              break;

          case ':':
              $suf = ' ';
              break;

          case ',':

              if (!$quote_state)
              {  
                  $suf = $ret;                                                                                                
                  $suf .= str_repeat($ind, $level);
              }
              break;

          case '}':
              $level--;   

          case ']':
              $pre = $ret;
              $pre .= str_repeat($ind, $level);
              break;

      }

      $beauty_json .= $pre.$data[$i].$suf;

  }

  return $beauty_json;

}
*/
file_put_contents($file, $data, LOCK_EX);
//file_put_contents($file, pretty_json($data), LOCK_EX);

?>
