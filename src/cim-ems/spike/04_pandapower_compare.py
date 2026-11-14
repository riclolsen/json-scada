import warnings; warnings.filterwarnings("ignore")
import pypowsybl.network as pn, pypowsybl.loadflow as lf, sys, pandas as pd
import pandapower as pp
from pandapower.converter.cim import from_cim
out=sys.argv[1]
for name in ["ieee14","ieee57"]:
    n=getattr(pn,"create_"+name)(); lf.run_ac(n)
    g=n.get_generators(); slack_bus=g.sort_values('target_p',ascending=False).bus_id.iloc[0]
    net=from_cim.from_cim(file_list=[f"{out}/{name}.zip"])
    print(name, net.bus.columns.tolist()[:10], net.gen.columns.tolist()[:12] if len(net.gen) else "nogen", len(net.ext_grid))
    # make the gen on the busiest-generation bus the slack
    pb=n.get_buses()
    net.gen['slack']=False
    # pick gen with largest p_mw
    k=net.gen.p_mw.idxmax(); net.gen.loc[k,'slack']=True
    pp.runpp(net, init="dc", calculate_voltage_angles=True, enforce_q_lims=False, numba=False, distributed_slack=False)
    print(" pp converged", net.converged, "buses", len(net.res_bus), len(pb))
    # match by bus name (CIM name) vs iidm name
    a=net.bus.reset_index().rename(columns={'index':'ppi'}).join(net.res_bus[['vm_pu','va_degree']],on='ppi')
    pbn=pb.reset_index().rename(columns={'id':'iid'})
    print(a[['name','vm_pu','va_degree']].head(3).to_string()); print(pbn[['iid','name','v_mag','v_angle']].head(3).to_string())
