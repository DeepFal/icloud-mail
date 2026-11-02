import assert from "node:assert/strict";
import test from "node:test";
import { getServices, saveMailboxNote, saveRegistration, getServiceMailboxes, matchesMailboxOption, registrationLabel } from "../src/workspace/registry-api.js";
test("mailbox selectors match literal addresses and notes without relying on generated labels", () => {
 const option = { label: "mail.23@Example.Test", note: "GitHub 工作账号 100%", secondary: "旧名称" };
 assert.equal(matchesMailboxOption(option, " example.test "), true);
 assert.equal(matchesMailboxOption(option, "工作账号"), true);
 assert.equal(matchesMailboxOption(option, "100%"), true);
 assert.equal(matchesMailboxOption(option, ".*"), false);
 assert.equal(matchesMailboxOption(option, ""), true);
 assert.equal(registrationLabel("unrecorded"), "未记录");
 assert.notEqual(registrationLabel("unrecorded"), registrationLabel("not_registered"));
});
test("registry API writes only local profile data with CSRF and preserves explicit blank notes", async (t) => {
 const calls=[];
 t.mock.method(globalThis,"fetch",async(url,options)=>{calls.push({url,options});return new Response(JSON.stringify({data:String(url).includes('/services?')?{items:[]}:{items:[],total:0}}),{headers:{'content-type':'application/json'}});});
 await getServices({query:"Open AI"});
 await saveMailboxNote(3,"","csrf-fixture");
 await saveRegistration(3,{service_id:2,status:"pending",account_identifier:"账号甲",note:"待人工确认"},"csrf-fixture");
 await getServiceMailboxes({serviceId:2,status:"unrecorded",query:"%_",limit:50});
 assert.match(calls[0].url,/query=Open\+AI/);
 assert.equal(calls[1].options.headers.get('X-CSRF-Token'),"csrf-fixture");
 assert.equal(calls[1].options.body,'{"note":""}');
 assert.equal(calls[2].options.method,"PUT");
 assert.equal(JSON.parse(calls[2].options.body).status,"pending");
 assert.match(calls[3].url,/status=unrecorded/);
 assert.match(calls[3].url,/query=%25_/);
});
