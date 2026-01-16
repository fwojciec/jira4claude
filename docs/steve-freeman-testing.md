# Steve Freeman's testing philosophy: design through dialogue

The core of Steve Freeman's philosophy is that **tests are not verification tools but design instruments**. Every difficult test is what Freeman calls a "design moment"—a signal that something in your code structure needs attention. This framing transforms testing from overhead into the primary feedback mechanism for evolving well-structured software. Freeman's approach, developed with Nat Pryce and codified in *Growing Object-Oriented Software, Guided by Tests* (GOOS), treats test friction not as an obstacle but as the most reliable early warning system for architectural problems.

---

## Tests that tell stories rather than prove correctness

Freeman fundamentally reframes what tests are for: "Tests should be a living documentation of what the software system does, they should be seen as examples of how our software should be used." In his 2014 interview with Johannes Link, he states: "Nowadays, I put a great deal of emphasis on making the tests readable and expressive, rather than asserting every last little detail; actually, I always did but now it's my top priority."

His one-line explanation of good TDD captures this: **"Think about what story the tests tell about the code. If you can't understand it, then either the code or the tests need to be improved, or possibly both."** This story-telling metaphor pervades his work. Test names should describe behavior, not mirror method names. The test structure should read as narrative—what situation existed, what action occurred, what resulted.

Freeman identifies six characteristics of an excellent test suite in an interview with Arlo Belshee:

- **Expressiveness**: "I can read any test and understand what point about the code I was trying to make at the time"
- **Confidence**: "If the tests pass, the system is pretty much good enough to ship"
- **Speed**: "It's a game changer"
- **Separation of concerns**: "Higher-level tests assume that lower-level code works"
- **Good diagnostics**: "When a test fails, the output tells me what went wrong"
- **Well-tended**: "I regularly review what I have and maintain them to reduce duplication and uncover a domain language"

The practical implication is treating test code with the same care as production code. The "Refactor" step in TDD should include refactoring test readability. Freeman emphasizes that concepts from well-written tests often migrate into production code, creating a virtuous cycle where the domain language emerges from testing.

---

## Listening to test difficulty as your primary design signal

Chapter 20 of GOOS, titled "Listening to the Tests," contains Freeman's most important philosophical contribution: "Sometimes we find it difficult to write a test for some functionality we want to add to our code. In our experience, this usually means that our design can be improved—perhaps the class is too tightly coupled to its environment or does not have clear responsibilities."

Michael Feathers, endorsing the book, crystallizes this: **"Every time you encounter a testability problem, there is an underlying design problem."** Freeman elaborates: "We've found that the qualities that make an object easy to test also make our code responsive to change."

Specific test smells Freeman and Pryce identified from years of experience that signal production code problems:

**Difficult test setup** indicates a class has too many dependencies or unclear responsibilities. **Need for complex mocking** suggests the object under test is doing too much work. **Tests breaking frequently** points to tight coupling to implementation rather than behavior. **"Train wreck" dependencies**—chains of getters like `dog.getBody().getTail().wag()`—violate the "Tell, Don't Ask" principle. **Static or global state** indicates poor isolation of concerns. **Flag arguments in constructors** signal poor cohesion.

In his GOTO Berlin 2013 talk "TDD: That's Not What We Meant," Freeman explains: "At the core of the practice is discovering a new test that is difficult to write. That's your design moment when you need to think about your current structure. Perhaps the test is telling you that the unit of code is getting too large, or that you need to expose more access for a new feature, or something else. **The important thing is to stop and reflect, and maybe refactor, rather than just pushing on somehow.**"

His practical advice: "Sensitise yourself to find the rough edges in your tests and use them for rapid feedback about what to do with the code. Don't stop at the immediate problem (an ugly test) but look deeper for why I'm in this situation (weakness in the design) and address that."

---

## Testing intent without over-specifying implementation

The GOOS principle is stark: **"Unit-test behavior, not methods."** These often differ significantly—a single unit of behavior may span multiple methods, and a single method may participate in multiple behaviors. Freeman's 2014 interview clarifies his approach: "I don't prioritise one over the other absolutely, I prioritise the one that's appropriate for the type of object being tested. If I have an object that has behaviour and I'm doing 'Tell, Don't Ask' then I can only test interactions."

The book distinguishes between internal and external quality. **External quality** addresses whether the system meets user needs—functional, reliable, responsive. **Internal quality** addresses whether code serves developers—easy to understand and change. Tests for external quality focus on **what** the system does; tests for internal quality ensure the **how** can evolve.

The "Tell, Don't Ask" principle operationalizes this distinction. From the OOPSLA 2004 paper: "Ask the question we really want answered, instead of asking for the information to help us figure out the answer ourselves." Rather than `dog.getBody().getTail().wag()`, write `dog.expressHappiness()` and let the implementation decide what this means.

The foundational mock objects paper states: "If objects interact by sending each other commands, their public interfaces provide no methods that let you interrogate their state. The only way to observe the behaviour of an object is to see how it affects the state of its world by sending commands to other objects. And that's what Mock Objects let you do."

This has a crucial implication: when following "Tell, Don't Ask" style, **you often have no postconditions to assert**. The test verifies that interactions occurred correctly, not that state changed. This isn't a weakness—it's by design. The behavior is in the communication pattern, not in observable state changes.

---

## Calibrating test sensitivity through deliberate precision

The "Mock Roles, Not Objects" paper addresses brittleness directly, paraphrasing Einstein: **"A specification should be as precise as possible, but not more precise."** Freeman identifies that "one of the risks with TDD is that tests become 'brittle'... A test suite that contains a lot of brittle tests will slow down development and inhibit refactoring."

The balance comes from several principles. First, **only mock types you own**: "Programmers should only write mocks for types that they can change. Otherwise they cannot change the design to respond to requirements." This means wrapping external libraries with your own abstractions and testing against those abstractions.

Second, **only mock your immediate neighbors**: "An object that has to navigate a network of objects in its implementation is likely to be brittle because it has too many dependencies." Deep chains of mocks are a design smell, not a testing problem.

Third, **be explicit about things that should not happen**: "A specification that a method should not be called is not the same as a specification that doesn't mention the method at all." Use `expect(never())` when absence of calls matters, but don't over-specify every possible non-call.

Fourth, **don't add behavior to mocks**: "Mock objects are still stubs and should not add any additional complexity. An urge to start adding real behaviour to a mock object is usually a symptom of misplaced responsibilities."

The practical guidance from jMock's design: use **constraints** rather than exact values to avoid over-specification. Allow loosening requirements where details are unimportant to the test being written. Tests should verify the essential contract, not every incidental implementation detail.

Freeman explicitly addresses the criticism that mock-heavy tests couple to implementation: "The communication pattern between classes is an implementation detail. The communication pattern only becomes part of API when it crosses the boundary of the system." Within a system, interaction patterns are internal; at system boundaries, they become contracts worth testing.

---

## Fractal TDD applies the same principles at every architectural level

Freeman's GOTO Aarhus 2011 talk "Fractal TDD: Using Tests to Drive System Design" extends unit-level thinking to system design: "TDD at the class level is now well understood (if not always well practiced). But the benefits from writing tests first and using them to drive design apply at the system level too."

The **walking skeleton** concept, borrowed from Alistair Cockburn, implements this: "An implementation of the thinnest possible slice of real functionality that we can automatically build, deploy, and test end-to-end." A walking skeleton must be:

- Automatically buildable
- Automatically deployable  
- Automatically testable
- Functional enough to exercise the architecture

Freeman explains why starting with this matters: "Acceptance tests must run end-to-end to give us the feedback we need about the system's external interfaces, which means we must have implemented a whole build, deploy, and test cycle to implement the first feature." The walking skeleton forces this infrastructure into existence early, when it's cheapest to get right.

**Ports and adapters architecture** enables this approach to scale. The GOOS approach: "When developing software in the GOOS TDD style we want to start with the system inputs and outputs and work towards the domain model." External dependencies get wrapped in adapters you own. Your code depends only on ports (interfaces) that you define.

This has direct testing implications. Freeman states: "Don't mock what you don't own or code that you cannot change. Consider writing wrapper classes that encapsulate these scenarios, such as repositories or proxies." Mock your ports in unit tests; use integration tests against real systems through your adapters.

The fractal nature emerges: at the acceptance test level, you're testing system behavior against external contracts. At the unit level, you're testing object behavior against collaborator contracts. The same principles—need-driven design, interface discovery, listening to test friction—apply at every scale.

Freeman addresses a key challenge: "Putting testing at the front of the development process flushes out architectural issues like concurrency and distribution. This results in systems that are easier to maintain AND support." The walking skeleton forces confrontation with deployment, distribution, and integration concerns before they become entangled with business logic.

---

## The discipline of tiny, stupid steps builds confidence incrementally

Freeman's approach to incrementalism connects to risk management. In his Agile City London talk, he quoted Alan Perlis from the 1968 NATO Conference: "A software system can best be designed if the testing is interlaced with the designing instead of being used after the design."

The double-loop TDD model from GOOS operationalizes this:

1. **Outer loop**: Write an acceptance test that fails
2. **Inner loop**: Use regular TDD to implement just enough code
3. Continue until the acceptance test passes
4. Each feedback loop addresses different concerns—inner loops handle technical detail, outer loops address organization and team effectiveness

Freeman explains the philosophy: "Each loop exposes the team's output to empirical feedback so that the team can discover and correct any errors or misconceptions. The nested feedback loops reinforce each other; if a discrepancy slips through an inner loop, there is a good chance an outer loop will catch it."

His blog post "Bad Code Isn't Technical Debt, It's an Unhedged Call Option" provides the risk framing: "Call options are a better model than debt for cruddy code because they capture the unpredictability of what we do. If I pop in a feature without cleaning up then I get the benefit immediately, I collect the premium. If I never see that code again, then I'm ahead. On the other hand, if a radical new feature comes in that I have to do, all those quick fixes suddenly become very expensive to work with. **The scary thing is that failure, if it comes, can be sudden—everything is fine until it isn't.**"

Small steps hedge against this unlimited downside. Each passing test is evidence that your structure works. Each difficult test is a signal to pause and improve structure before proceeding. "Refactoring is like buying an option too. I pay a premium now so that I have more choices about where I might take the code later."

The practical discipline: resist the temptation to push through difficult tests. **Stop and reflect.** The friction you're feeling is information. Use it.

---

## Designing for testability yields operational benefits

Freeman's connection between testability and production quality emerges from the fundamental insight that **the qualities making code testable are the qualities making code changeable and maintainable**.

From his mockobjects.com writing: "TDD with mocks encourages me to write objects that tell each other what to do, rather than requesting and manipulating values. Applied consistently, I end up with a coding style where I pass behaviour (in listeners and callbacks) from the higher levels of the code to the data, rather than pulling data up through the stack."

The operational implications: "It's an unusual style in the Java/C# world, but I find that we get better encapsulation and clearer abstractions because I have to clarify what the pieces will do, not just what values they hold." Objects designed this way are naturally more observable—their behavior is expressed through interactions that can be monitored.

Freeman adds a fourth step to the classic Red-Green-Refactor cycle: **"Clearing up the diagnostic message."** When tests fail, the failure message should explain what went wrong without requiring a debugger. This discipline creates systems where failures are inherently informative—a quality that transfers directly to production monitoring and debugging.

From his interview material: "When tests fail, the output tells me what went wrong. I don't have to spend a week with the debugger to figure out what happened." This diagnostic clarity isn't just a testing convenience; it's a design quality that makes systems more supportable in production.

The connection to ports and adapters reinforces this. When external dependencies are wrapped in adapters you control, you gain natural injection points for monitoring, logging, and operational instrumentation. The same seams that enable testing enable observability.

---

## The mental model beneath the techniques

Freeman's philosophy ultimately rests on a view of software articulated in Kent Beck's foreword to GOOS: "What if software wasn't 'made,' like we make a paper airplane—finish folding it and fly it away? What if, instead, we treated software more like a valuable, productive plant, to be nurtured, pruned, harvested, fertilized, and watered?"

This gardening metaphor shapes everything. Tests aren't quality gates at the end of a manufacturing process; they're ongoing feedback in a cultivation process. Design isn't something completed before implementation; it emerges through the discipline of listening to what tests reveal.

Freeman's practical questions when writing tests become:

1. **What story does this test tell?** Can someone reading it understand the point I'm trying to make?
2. **Is this test difficult to write?** If so, what design improvement would make it easier?
3. **What role does this collaborator play?** Not what object is it, but what responsibility does it have in this interaction?
4. **Am I testing behavior or implementation?** Would this test survive a legitimate refactoring?
5. **Am I specifying what matters?** Is this constraint essential or incidental?

The deepest insight may be this recognition of TDD as a thinking tool: "I've come to a better understanding of the use of TDD as a 'thinking tool', to help me clarify my ideas before coding." Tests don't just verify ideas—they force you to have ideas precise enough to verify.

Freeman's warning about incomplete adoption rings through his talks: "TDD is a deep skill, like anything in programming. A couple of days of training and a flip through a book is no more than a taster." The mental models matter more than the mechanics. Without understanding why tests tell you about design, the techniques become ritual rather than revelation.

## Conclusion

Freeman's contribution isn't a testing technique—it's a way of thinking about software evolution where **tests are conversations with your future self and your future code**. Test friction isn't obstacle but oracle. Mockobjects aren't isolation devices but role-discovery tools. The walking skeleton isn't scaffolding but foundation.

The heuristic that captures it all: when a test is hard to write, you've found the most valuable moment in your development process. Stop. Think. The test is telling you something your code cannot say for itself.
