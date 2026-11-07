import pypowsybl.network as pn, pypowsybl.loadflow as lf, sys, pandas as pd
pd.set_option("display.width",250)
n=pn.create_four_substations_node_breaker_network(); lf.run_ac(n)
n2=pn.load(sys.argv[1]+"/exp_nb.zip"); lf.run_ac(n2)
for f in ["get_vsc_converter_stations","get_static_var_compensators","get_generators","get_shunt_compensators"]:
    a=getattr(n,f)(); c=getattr(n2,f)()
    cols=[x for x in a.columns if x in c.columns and a[x].dtype!=object]
    d=(a[cols].astype(float)-c[cols].reindex(a.index).astype(float)).abs().max()
    print(f, "\n", d[d>1e-6].to_string() if (d>1e-6).any() else "identical")
print(n.get_vsc_converter_stations()[['target_q','target_v','voltage_regulator_on','q','bus_id']]); print(n2.get_vsc_converter_stations()[['target_q','target_v','voltage_regulator_on','q','bus_id']])
