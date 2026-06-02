const fs = require('fs');
const path = require('path');

const dir = path.join(__dirname, 'cmd', 'director', 'dummy_submissions');

const questions = {
  q1: {
    logic: `const arr = input.split(',').map(Number); arr.sort((a,b)=>a-b); console.log(arr.join(','));`,
    waLogic: `console.log("0");`
  },
  q2: {
    logic: `const [a,b] = input.split(' ').map(Number); console.log((a+b)/2);`,
    waLogic: `console.log("-999");`
  },
  q3: {
    logic: `const lines = input.split('\\n'); const arr = lines[0].split(' ').map(Number); const target = Number(lines[1]); console.log(arr.indexOf(target));`,
    waLogic: `console.log("false");`
  },
  q4: {
    logic: `const [a,b] = input.split(' ').map(Number); console.log(a % b);`,
    waLogic: `console.log("1000");`
  },
  q5: {
    logic: `console.log(Math.log2(Number(input)));`,
    waLogic: `console.log("-1");`
  }
};

const templates = {
  fast_ac: (q) => `const fs = require('fs'); const input = fs.readFileSync(0, 'utf-8').trim(); ${questions[q].logic}`,
  slow_ac: (q) => `const fs = require('fs'); const input = fs.readFileSync(0, 'utf-8').trim(); setTimeout(() => { ${questions[q].logic} }, 1000);`,
  wa: (q) => `const fs = require('fs'); const input = fs.readFileSync(0, 'utf-8').trim(); ${questions[q].waLogic}`,
  tle: (q) => `while(true){}`,
  mle: (q) => `const a = []; while(true) a.push(new Array(1000000).fill(1));`
};

if (!fs.existsSync(dir)) fs.mkdirSync(dir, {recursive: true});

for (const q of Object.keys(questions)) {
  for (const type of Object.keys(templates)) {
    fs.writeFileSync(path.join(dir, `${q}_${type}.js`), templates[type](q));
  }
}
console.log("Done generating 25 dummy submissions.");
