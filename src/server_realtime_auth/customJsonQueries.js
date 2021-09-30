const GetQueryPostgresql = function (queryName) {
  switch (queryName) {
    default:
      return ''
    case 'notes':
      return `
        select 
          tag,          
          json_data->>'_id' as _id,
          json_data->>'description' as description,
          json_data->>'notes' as notes,
          json_data->>'annotation' as annotation
        from realtime_data 
        where json_data->>'notes' != '' or json_data->>'annotation' != ''
        order by tag
        `
    case 'alarmDisabled':
      return `
        select 
          tag,          
          json_data->>'_id' as _id,
          json_data->>'description' as description,
          json_data->>'alarmDisabled' as alarmDisabled
        from realtime_data 
        where json_data->>'alarmDisabled' = 'true'
        order by tag
        `
      break
    case 'desligamentos':
      return `
      SELECT 
        last(s.tag, s.time_tag) as tag
        ,CASE d.json_data->>'type' WHEN 'digital' THEN CASE last(s.value, s.time_tag) WHEN 0 THEN d.json_data->>'stateTextFalse' ELSE d.json_data->>'stateTextTrue' END WHEN 'analog' THEN to_char(last(s.value, s.time_tag), 'FM999990D0') END as status
        ,d.json_data->>'unit' as unidade
        ,case (last(s.flags, s.time_tag)::bit(8)>>7)::bit(1) when B'1' then 128 ELSE 0 END as flags
        ,CASE d.json_data->>'type' WHEN 'digital' THEN 'D' ELSE 'A' END as tipo
        ,substr(d.tag,1,9) as mod
        -- ,last(s.value, s.time_tag) as estado
        -- ,d.json_data->>'stateTextTrue' as estado_on
        ,max(s.value) as valor_max
        ,CASE (d.tag like '%XCBR%' and d.json_data->>'stateTextFalse'='DESLIGADO') WHEN TRUE THEN 1 ELSE 0 END as ehdj
        ,d.json_data->>'group1' as estacao
        ,d.json_data->>'group2' as modulo
        ,d.json_data->>'ungroupedDescription' as descricao
        ,substring(d.tag, 1, 9) as cod_modulo
        ,(d.json_data->'priority')::int as prioridade
        ,to_char(last(s.time_tag, s.time_tag) at TIME ZONE 'UTC+3','HH24:MI') as ts
        ,(extract(epoch from last( s.time_tag, s.time_tag)))::int as unixts
        ,last(s.time_tag_at_source, s.time_tag_at_source) as tss
        ,(extract(epoch from NOW()) - extract(epoch from last( s.time_tag, s.time_tag)))::int as tempo
      FROM   hist s
      JOIN   realtime_data d on d.tag=s.tag
      WHERE 
      s.time_tag > (NOW() + interval '-1 hours')  
      and ( d.json_data->>'type' = 'digital' and (d.json_data->'priority')::int = 0 
            or
        d.json_data->>'type' = 'analog' and d.json_data->>'unit' in ('kV', 'MVA', 'MW', 'Mvar')
          )  
      -- and d.json_data->>'type' = 'digital'
     and CONCAT(d.json_data->>'group1',d.json_data->>'group2') in 
     (
     SELECT estmod FROM
     ( -- procura todos os eventos de DJ
     SELECT 
     CONCAT(d.json_data->>'group1',d.json_data->>'group2') as estmod
     ,last(s.value, s.time_tag) as valor
     -- ,(d.json_data->'_id')::int as nponto
     -- ,d.tag as id
     FROM   hist s
     JOIN   realtime_data d on d.tag=s.tag
     WHERE  
     s.time_tag_at_source is not null 
     and s.tag like '%XCBR%'
     and s.tag not like '%XCBR24%'
     and (d.json_data->'value')::int = 0 -- estado desligado
     and (d.json_data->'alarmed')::boolean = true
     and d.json_data->>'type' = 'digital'
     and (d.json_data->'priority')::int = 0 
     and d.json_data->>'stateTextTrue' = 'LIGADO'
     and d.json_data->>'stateTextFalse' = 'DESLIGADO'
     and s.time_tag > (NOW() + interval '-1 hours')
     and d.json_data->>'description' not like '%Serv.Aux.%'
     and d.json_data->>'description' not like '%Serv. Aux.%'
     GROUP BY d.tag
     ) hh WHERE valor = 0 -- fica somente com os eventos que desligaram
     )
     GROUP BY d.tag
     ORDER BY estacao, modulo, unidade, tag desc
  
        `
  }
}

module.exports = GetQueryPostgresql
