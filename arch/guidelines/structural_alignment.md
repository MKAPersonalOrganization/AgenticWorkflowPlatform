Architecture, Source Code, and Structural Alignment
Why Alignment Matters
Consider a typical 10-story office building. A building architect is responsible for that building's structural integrity (its architecture). To ensure the building will be structurally sound, the architect, together with the structural engineer, takes into account loads, spans, stresses, weather, ground topology, building materials, and a variety of other factors. They document all of this in the building plans, which the construction contractors use to construct (or implement) the architecture. Those who work in the completed office building assume that the building plan was implemented according to the specifications and constraints the architect defined, so that the ceiling won't suddenly collapse or the walls suddenly buckle under extreme load.

Now consider a software system or product. It also has an architect--a software architect--who creates a structure (an architecture) to ensure that the system meets certain critical capabilities, such as scalability, responsiveness, availability, fault tolerance, and so on. The software developers function like a construction crew, implementing the architectural solution through source code. Like the office building, the people who work in this system assume it was implemented according to the specifications and will meet all of the architectural characteristics and business concerns defined by the business stakeholders. If not, then the system will likely come crashing down like a poorly constructed 10-story office building.

Aligning a system's implementation and its architecture is critical to that system's success. If they become misaligned, the system will fail to achieve its goals, and in some cases simply stop working. Unfortunately, this misalignment is fairly common. Let's look at some of the reasons why:

Communication and collaboration

Software architects and developers don't communicate and collaborate nearly enough. Developers complain that architects are never around and are too busy to answer their questions and issues. Architects complain that developers don't pay enough attention to the architecture and end up developing code that doesn't focus on the critical architectural characteristics. This lack of communication and collaboration usually results in the implementation becoming misaligned with the architecture.

Documentation falling out of sync

Source code is constantly changing as developers fix bugs, add new features, update features, and pay off technical debt. Sometimes those changes affect the structure--the architecture--of the system. When that happens, most of the time the corresponding architecture diagrams and documentation are not updated to reflect those source code changes, so the implementation and architecture become misaligned. Keeping the implementation of a system aligned and in sync with its architecture (and vice versa) is an ongoing challenge in most organizations.

Slow feedback loops

When development teams modify source code, they sometimes modify the corresponding architecture of that system as well. Unfortunately, the feedback loop for architectural changes made through source code is usually too slow, so the architect of that system often doesn't realize soon enough (if ever) that the underlying architecture has been changed, resulting in a misalignment.

Lack of architectural understanding

Many development teams tasked with implementing or maintaining a system are unaware of its underlying software architecture. This may be due to a lack of communication and collaboration from the architect (the first challenge in this list), high levels of attrition and reassignment within the development team, or even legacy systems that have no architectural documentation (or even no architecture). In these situations it's impossible for the development team to implement or maintain the architecture correctly, causing it to become misaligned and not satisfy the system's business and architecture characteristics goals.

Architecture and Source Code
Software architecture represents the logical and physical structure of a software system or product. The logical architecture of a software system identifies relationships between its components and how the source code is structured, whereas the physical architecture represents the deployed artifacts of the system, such as services, gateways, message brokers, and so on.

How a system is implemented can affect its architecture, and hence the corresponding architectural characteristics it must support. For example, if a given system requires high levels of scalability and elasticity, its architect must take care to minimize the amount of internal memory the services use, which can affect the system's ability to create additional instances of that service based on the amount of memory available in the virtual machine. A 500MB internal in-memory cache contained in a service instance might not seem like a lot, but when 10 instances of that service start up, that translates to a total memory consumption of 5GB. It's 10GB after 20 service instances. Imagine a system with 25 services, each containing a 500MB internal in-memory cache, with eight instances of each running. That translates to 25,000GB of required memory, which greatly exceeds the memory of most physical hardware.

In this chapter, we revisit the concept of architectural components, which we've also covered in our books Fundamentals of Software Architecture and Head First Software Architecture. Here, we show how the structure of source code reflects the components, subdomains, and domains of its software architecture.

Use Case: The Sysops Squad
In our 2022 book Software Architecture: The Hard Parts, we illustrated concepts using the Sysops Squad, a complex trouble-ticket system for Penultimate Electronics, Inc, a fictional electronics retailer. We'll use the same use case in this part of the book. If you haven't read it, here's how their system works: When customers purchase electronic equipment such as computers, home entertainment systems, and stereo systems from the company's stores throughout the country, they can also choose to purchase a support plan. Then, if they run into issues with installation or the equipment, they can create a trouble ticket on the website and an assigned field expert will come to their home or office to fix the problem.

The Sysops Squad ticketing system's functionalities include reporting, ticket maintenance, ticket scheduling, customer sign-in, customer registration, billing, expert knowledge-base search, and maintenance. To limit the scope, for now we'll focus just on the core ticketing process and the customer survey functionality. The following actions illustrate the overall major workflow of this part of the Sysops Squad system:

The customer creates a trouble ticket using the public-facing Sysops Squad website
The system locates a nearby expert and assigns the ticket (this might take up to a couple of days)
Once an expert is available and assigned, the system notifies the customer that the expert is on their way
The system uploads the ticket to the expert's mobile device
The expert fixes the problem and marks the ticket as complete
The customer receives a survey link via email
The customer fills out and submits the survey using the public-facing Sysops Squad website
Sysops Squad Primary Ticketing Workflow Figure 2-1. The primary ticketing workflow for the Sysops Squad system

The corresponding logical architecture for the Sysops Squad system, based on this primary ticketing workflow, is illustrated in Figure 2-2. Here, solid lines represent synchronous communication and dotted lines represent asynchronous communication (we use this throughout the book). The logical architecture consists of two domains: Ticketing and Survey. The Ticketing domain contains two components, Ticket Creation and Ticket Completion, as well as an Assignment subdomain with three components: Ticket Assignment, Customer Notification, and Ticket Routing. The Survey domain contains three components: Send Survey, Receive Survey, and Survey Templates. Customers interact with the Survey domain and the Ticket Creation component, and the field experts interact with the Ticket Routing and Ticket Completion components.

Sysops Squad Logical Architecture Figure 2-2. The logical architecture for the Ticketing and Survey domains

We'll be using this logical architecture to describe the intersection of architecture and implementation, and how to govern that intersection using architecture as code.

Architectural Domains, Subdomains, and Components
Architectural components are the building blocks of software architecture. They represent some sort of behavior within the system and are implemented through source code. Components are contained within domains and/or subdomains in the system. In most systems, these artifacts are realized through the directory structures or namespaces of the source code.

As illustrated in Figure 2-3, the higher-level directory structures or namespaces of the source code represent the domains and subdomains. In this example, the Ticketing domain is represented through the higher-level ticketing directory, the Assignment subdomain under the Ticketing domain is represented through the assignment directory, and the Survey domain represented through the higher-level survey directory.

Architecture and Source Code Structure: Domains Figure 2-3. Domains and subdomains are represented through higher-level directories or namespaces

Logical components, which are those building blocks of a system that contain some sort of business or infrastructure functionality, are represented through the leaf nodes of the directory or namespace structure, as shown in Figure 2-4.

Logical components are represented through leaf-node directories or namespaces Figure 2-4. Logical components are represented through leaf-node directories or namespaces

This concept of representing a system's architecture through its source code's directory or namespace structure is a critical part of how architects can govern systems to ensure alignment between the architecture and the corresponding implementation.

Services
In distributed architectures, the directory/namespace concept still applies, since each separately deployed service has an internal architecture made up of components (and possibly subdomains, depending on the size of the service). However, this concept can also be used to define the physical architecture. Using the same directory structure as in a monolith, each higher-level directory or namespace node can represent a separately deployed service containing logical components. For example, in Figure 2-5 we've identified three separately deployed services: Ticketing, Assignment, and Survey. Here the higher-level nodes of the directory structure represent the service, and the leaf nodes the corresponding logical components or subdomains contained in those services.

Services can be represented through higher-level directories or namespaces Figure 2-5. Services can be represented through higher-level directories or namespaces

We'll be using these examples throughout this and the following chapter to describe how implementation can get misaligned with the architecture, and how to govern this intersection using architecture as code to ensure proper alignment.

Implementation Misalignment
It is quite common for implementation and architecture to become misaligned. The reasons are plentiful: Architects are generally responsible for creating and maintaining the logical and physical architecture of a system, and development teams are responsible for implementing that architecture. Unfortunately, what happens all too often is that development teams only use architecture diagrams to gain an understanding of the architecture's vision, then largely ignore them during implementation and maintenance. The reverse is also true--architects are generally unaware of changes to the logical and physical architecture based on the implementation, so the architecture diagrams and other artifacts quickly get out of sync with the implementation.

To illustrate this point, consider the directory structure shown in Figure 2-6, which represents how the development teams decided to implement the Sysops Squad logical architecture, as shown in Figure 2-2.

Sysops Squad Misaligned Implementation Figure 2-6. The resulting Sysops Squad directory structure created by the development team

Notice that the higher-level directories don't match the domains and subdomains of the defined logical architecture illustrated in Figure 2-2 (Ticketing, Assignment, and Survey), nor do the leaf nodes represent the corresponding logical components match. As a matter of fact, given this code structure, the development team came up with the logical architecture shown in Figure 2-7.

The Sysops Squad logical architecture created by the development team Figure 2-7. The Sysops Squad logical architecture created by the development team

Even though this architecture passes all of the functional unit and scenario tests, it is misaligned with its implementation, which therefore risks not supporting the architectural characteristics the system requires. Comparing the architecture illustrated in Figure 2-2 with that of the resulting implementation in Figure 2-7, the negative implications of this kind of misalignment become apparent. These include:

Agility

Agility is the ability to respond quickly to change. It's made up of three primary architectural characteristics: maintainability, testability, and deployability. Maintainability describes how easy it is for a developer to locate where in the code to apply a change, fix a bug, or add a new feature. Testability describes the ease of and completeness of testing and the risk of introducing bugs as a result of changing or adding code. Deployability is about the ceremony, frequency, and risk of releasing the software.

Of the Sysops Squad's two logical architectures, the original one created by the software architect is more maintainable, testable, and less risky to deploy--and thus more agile--than the one created from the implemented directory structure, because the original architecture is less dynamically decoupled, clearer, and more understandable.

Reliability

Reliability is a composite architectural characteristic, or one made up of several other characteristics--in this case, availability, testability (for bugs and errors), data integrity, data consistency, and fault tolerance. Of the two logical architectures for the Sysops Squad, the original one created by the software architect is more reliable than the one created through implementation because there is less dynamic coupling between components and it contains more definable components that the implemented version, making it easier for developers to locate, apply, and test changes.

Adaptability

Adaptability is the system's ability to adapt to changes in its environment, platform, and functionality. The original logical architecture is more adaptable than the one created by the development team because the components and their interactions are more clearly defined, allowing developers to more easily identify areas of the architecture that are impacted by these kind of changes.

Extensibility

Extensibility is the ease with which the architecture can be extended to include additional functionality and features like additional domains, subdomains, and logical components. Of the two architectures, the one created by the development team through the structure of the source code is less extensible than the original, because the more defined the components and their clear interactions, the easier it is to extend the architecture with additional functionality.

Migration

Migration means converting a legacy system (usually a monolithic architecture) to a distributed architecture (such as microservices or event-driven architecture). The original architecture would be much easier to migrate than the one created by the development team because the logical components and their boundaries are better defined, allowing developers to better identify and create services based on those components.

It's important to note that this kind of misalignment doesn't mean the development team was acting irresponsibly when implementing the architecture. Rather, it usually stems from a lack of communication and collaboration between software architects and development teams, which generally results in the development teams not fully understanding the architecture. It can also result from not having enough automated governance in place to detect when the implementation doesn't match the architecture. Architectural diagrams are great for conveying ideas and concepts and for documenting the architecture of a system, but they aren't easy to enforce.

In the next chapter, we'll show some techniques for using architecture as code (AAC) and Architecture Definition Language (ADL) pseudo-code to detect when the implementation becomes misaligned with the architecture and to keep them aligned. These techniques also foster better communication and collaboration between software architects and development teams while also providing software documentation.

Structural Alignment
Defining Domains and Services
Domains in monolithic architectures (those with a single deployment unit) are typically defined as the highest-level grouping of components. They represent a major part of the system from a functionality standpoint. For example, the Sysops Squad logical architecture, depicted in Figure 2-2 in Chapter 2, consists of two domains: Ticketing and Survey. Many other domains exist in this system, such as Reporting and Analytics, Administration, and Expert Knowledge Base, but we've chosen to limit our example to the primary ticketing flow for simplicity's sake.

In distributed systems (those with separate deployment units), domains are often described as services. The highest-level grouping in the directory structure (or namespace) represents a separately deployed service, depending on what kind of code repository, or "repo," the teams are using: a mono-repo (single repo), one repo per domain, or one repo per service. The following sections describe each of these scenarios.

Monolithic architectures
Architects can describe the domains of a monolithic system by using the Architecture Definition Language (ADL), which we introduced in [Link to Come] in Chapter 1, to build pseudo-code. They can feed this pseudo-code to a generative AI tool to create executable source code to govern those domains in the system. For example, the following ADL pseudo-code in Example 3-1 defines the domains for the primary ticketing portion of the Sysops Squad system.

Example 3-1. ADL for describing the domains in Sysops Squad

DESCRIPTION Define Domains
CATEGORY Structural
DEFINE SYSTEM Sysops Squad AS sysops
  DEFINE DOMAIN Ticketing AS ticketing
  DEFINE DOMAIN Survey AS survey
ASSERT(CLASSES are only CONTAINED within SUBDOMAINS within DOMAINS)
The DESCRIPTION and CATEGORY lines are part of the metadata used to describe the context of the ADL. The DEFINE verb is used to define a particular artifact. It takes a logical name and associates it with the name of the corresponding physical directory or namespace. For example, in the DEFINE SYSTEM Sysops Squad AS sysops line, the Sysops Squad part describes the logical name of the system, and the sysops part describes the physical location, represented as either a directory or namespace. Given the DEFINE parts of this ADL, we can expect the matching directory structure (or namespace structure) to look like the one illustrated in Figure 3-1.

This code structure matches the domains defined for Sysops Squad Figure 3-1. This code structure matches the domains defined for Sysops Squad

The ASSERT part of the ADL specifies the test condition. In this case, the ADL is asserting that all classes must exist somewhere under these two directory structures (which may include subdirectories representing subdomains), but nowhere else. Notice that here the ADL is only defining the domains, not any of the other subdirectories defining the components. That will come later in the chapter.

Architects can prompt an LLM to generate executable source code from pseudo-code in a variety of languages and tools (listed in [Link to Come]). For example, giving an LLM the ADL in Example 3-1 and prompting it to generate an ArchUnit test in Java would yield the following code (abbreviated for brevity):

private static final String BASE_PACKAGE = "sysops";
private static final String TICKETING_PACKAGE = BASE_PACKAGE + ".ticketing..";
private static final String SURVEY_PACKAGE = BASE_PACKAGE + ".survey..";

ArchRule domainContainmentRule = ArchRuleDefinition.classes()
   .that().resideInAPackage(BASE_PACKAGE + "..")
   .should().resideInAnyPackage(TICKETING_PACKAGE, SURVEY_PACKAGE);

domainContainmentRule.check(importedClasses);
An architect could plug this executable test into a unit-testing framework like JUnit or into a CI/CD pipeline. Then, if a developer were to create a new high-level directory or namespace (essentially creating a new domain in the architecture), this test would fail, triggering a discussion.

Why not use a single ADL file to describe the entire architecture?
We often get questions about the granularity of the ADL files: is it better to use a single ADL file to describe the entire architecture, or separate ADL files to describe the domains and subdomains? While it might be tempting to use a single ADL file to describe an entire architecture, we advise against it for a variety of reasons.

First, when an architect uses an LLM to generate executable code from an ADL file, the output will be a single executable test class file, usually with each assertion contained in a separate method. We've found through practice that, while generative AI is really good at generating executable source code from a small, contained ADL file, it falters when the file gets too big and complex. This can happen if there's lots of nesting and indentation describing each domain, subdomain, and components within the domains and subdomains, as well as component dependencies.

Second, when architects and development teams add or remove a domain, subdomain, or component, the entire executable fitness function must be regenerated or directly modified, possibly including tweaking code generated from an ADL. Architects can end up spending a lot of time unnecessarily modifying the executable source code for the parts that didn't change. Therefore, we usually recommend breaking up the ADL files describing an architecture. Create one ADL file describing the domains in the system, then one or more ADL files describing each domain. "Defining Domain Details" provides additional guidance about ADL granularity within a domain definition.

Distributed architectures
In distributed architectures, each domain is often deployed as a separate service (as is typically the case with service-based architecture), or else broken apart into separately deployed services (as is the case with microservices).1 In the case of domain-based services, each domain would be represented as a service, so the ADL would use the keyword SERVICE rather than DOMAIN to define the higher-level structure of the system. This is shown in Example 3-2.

Example 3-2. ADL for describing domain services in Sysops Squad with a mono-repo

DESCRIPTION Define Services
CATEGORY Structural
DEFINE SYSTEM Sysops Squad AS sysops
  DEFINE SERVICE Ticketing AS ticketing
  DEFINE SERVICE Survey AS survey
ASSERT(CLASSES are only CONTAINED within SUBDOMAINS within SERVICES)
This ADL would only work when teams are using a mono-repo for their distributed architecture. The previous prompt and resulting source code for using domains would be the same when using services--any source code outside of these domain-based services would fail the executable fitness function. The same would be true for using the microservices architecture with a mono-repo, except that architects would generally revert back to the DOMAIN definition, then define services within that domain. (See "Defining Domain Details" for a deeper discussion of this.) However, if using a repo-per-domain or a repo-per-service, architects would generally skip this ADL unless for documentation purposes.

Defining Domain Details
While defining the domains of a system is fairly straightforward, defining how many ADL files to use for the details of each domain gets tricky. Architects have two primary options here: * Use a single ADL file to describe the entire domain, including components directly under the domain, subdomains, components within subdomains, and the corresponding dependencies between them * Describe the domain in terms of components and subdomains, then further define each subdomain through separate ADL files

ADL treats monolithic architectures differently than distributed ones when describing the structure of the system's domains. For monolithic architectures or distributed architectures that use domain services, the ADL describes the components and their interactions (dependencies). For distributed architectures such as microservices or event-driven architecture, the ADL describes the services within the domain as well as the components of those services. The following sections describe these options for monolithic architectures and distributed architectures.

Monolithic Architectures and Domain Services
Describing the entire domain in a single ADL file works only for monolithic systems or distributed systems using either a mono-repo or a repo-per-domain. For example, in the logical architecture illustrated in [Link to Come], the Ticketing domain consists of two components (Ticket Creation and Ticket Completion) and one subdomain called Assignment, which has three components (Ticket Assignment, Ticket Routing, and Customer Notification). The Ticket Creation component has a dependency on the Ticket Assignment component, and the Ticket Assignment component has a dependency on the Ticket Routing and Customer Notification components. This means that classes within the Ticket Creation component instantiate classes and invoke methods within the Ticket Assignment component, while classes within the Ticket Assignment component instantiate classes and invoke methods within the other Assignment subdomain components.

The logical architecture for the ticketing domain defined for Sysops Squad Figure 3-2. The logical architecture for the ticketing domain defined for Sysops Squad

Based on this logical architecture, Figure 3-3 shows the expected corresponding directory or namespace structure.

The complete directory structure for the ticketing domain for Sysops Squad Figure 3-3. The complete directory structure for the ticketing domain for Sysops Squad

Putting these two artifacts together, an architect can now write ADL (shown in Example 3-3) that describes the entire Ticketing domain of the Sysops Squad system, including all the subdomains, components, and component dependencies:

Example 3-3. ADL for describing the ticketing domain in Sysops Squad

DESCRIPTION Define Ticketing Domain
CATEGORY Structural
DEFINE SYSTEM Sysops Squad AS sysops
  DEFINE DOMAIN Ticketing AS ticketing
    DEFINE COMPONENT Ticket Creation AS creation
    DEFINE COMPONENT Ticket Completion AS completion       ❶
    DEFINE SUBDOMAIN Assignment AS assignment
      DEFINE COMPONENT Ticket Assignment AS assign_expert
      DEFINE COMPONENT Ticket Routing AS routing
      DEFINE COMPONENT Customer Notification AS customer_notification

ASSERT(CLASSES are only CONTAINED within COMPONENTS and SUBDOMAINS)   ❷

ASSERT(Ticket Creation IS ONLY DEPENDENT ON Ticket Assignment)        ❸
ASSERT(Ticket Completion has NO DEPENDENCY ON other components)
ASSERT(Ticket Assignment IS ONLY DEPENDENT ON Ticket Creation, Ticket Completion)
ASSERT(Ticket Routing has NO DEPENDENCY ON other components)
ASSERT(Customer Notification has NO DEPENDENCY ON other components)
❶ The top part of the ADL describes the structure of the Ticketing domain, defining each component, subdomain, and components under the subdomain. The AS keyword in the DEFINE verb specifies the physical location of that artifact (the directory node, namespace node, or package node where it is located), whereas the first part of the DEFINE verb describes the logical name. The definitions in this part of the ADL match the corresponding directory structures in Figure 3-3, where the highest-level directory defines the domain, the subdirectories define the subdomains, and the leaf nodes define the components. Also, in the definition for the Ticket Assignment component, the logical name (Ticket Assignment) is different from the physical name (assign_expert). This is common, since development teams usually name the physical directory or namespace node, whereas architects usually specify the logical name. It's the ADL that bridges these two and connects the logical name of a component, which shows up in diagrams and documentation, with the implementation name, which shows up in directories or namespaces.

❷ The first assertion states that all code must be within the components and subdomains defined in the ADL. This assertion enforces the specific structure of the directories and namespaces so that they correspond to the domains, subdomains, and components in the architecture.

❸ These assertions describe and govern component dependencies, which match those depicted in the logical architecture diagram shown in Figure 3-2.

Like with the domain definitions, an architect can prompt an LLM to use this ADL pseudocode to generate executable source code in a variety of languages, using specific testing libraries (see [Link to Come]). For example, prompting an LLM to generate ArchUnit code in Java using the ADL shown in Example 3-3 would produce generated Java code for each assertion. For the first assertion of the corresponding ADL in Example 3-3, the LLM would generate the following code (abbreviated for brevity):

String[] allowedPackages = {
    CREATION,
    COMPLETION,
    ASSIGN_EXPERT,
    ROUTING,
    CUSTOMER_NOTIFICATION
};

ArchRule containmentRule = noClasses()
    .that().resideInAPackage("..ticketing..")
    .should().resideOutsideOfPackages(allowedPackages);

containmentRule.check(importedClasses);
The same prompt and ADL could be used to generate NetArchtest code in C# as well:

private readonly string[] AllDomainNamespaces = {
    CreationNs,
    CompletionNs,
    AssignExpertNs,
    RoutingNs,
    CustomerNotificationNs
};

var result = Types.InCurrentDomain()
   .That().ResideInNamespace(TicketingRoot, true)
   .Should().ResideInNamespace(AllDomainNamespaces)
   .GetResult();

Assert.True(result.IsSuccessful, result.GetFailingTypes());
For the Ticket Creation dependency assertion in the ADL, the LLM would generate the following ArchUnit code in Java:

// Must depend on Ticket Assignment
ArchRule mustDependOnAssignment =
  classes().that().resideInAPackage(CREATION)             // ❶
           .should().dependOnClassesThat()
           .resideInAnyPackage(ASSIGN_EXPERT);

// Must NOT depend on any other component
ArchRule mustNotDependOnOthers =
  noClasses().that().resideInAPackage(CREATION)           // ❷
             .should().dependOnClassesThat()
             .resideInAnyPackage(
               COMPLETION,
               ROUTING,
               CUSTOMER_NOTIFICATION
);

mustDependOnAssignment.check(imported);
mustNotDependOnOthers.check(imported);
❶ Notice how the corresponding generated code enforces that the Ticket Creation component depends on the Ticket Assignment component. However, this code alone is not enough to enforce component dependencies because the Ticket Creation component could easily invoke classes in other components.

❷ This restricts the Ticket Creation component from invoking classes in other components. Fortunately, the ADL does not need to get this specific; the architect only needs to state direct dependencies. (For example: Ticket Creation IS ONLY DEPENDENT ON Ticket Assignment.) This implies that no other dependencies are allowed.

The Ticket Assignment dependency assertion denoted in the last part of the ADL produces similar generated code that includes the required component dependencies and excludes others:

// Must depend on Routing and Customer Notification
ArchRule mustDependOnAllowed =
  classes().that().resideInAPackage(ASSIGN_EXPERT)
           .should().dependOnClassesThat()
           .resideInAnyPackage(
              ROUTING,
              CUSTOMER_NOTIFICATION
);

// Must NOT depend on any other component
ArchRule mustNotDependOnOthers =
  noClasses().that().resideInAPackage(ASSIGN_EXPERT)
             .should().dependOnClassesThat()
             .resideInAnyPackage(
                CREATION,
                COMPLETION
);

mustDependOnAllowed.check(imported);
mustNotDependOnOthers.check(imported);
Finally, the other component dependency assertions denoted in the last part of the ADL specify no other component dependencies. The Java code generated for these assertions is as follows:

// Ticket Completion has NO dependencies on any other component
ArchRule completionNoDeps = noClasses()
        .that().resideInAPackage(COMPLETION)
        .should().dependOnClassesThat()
        .resideInAnyPackage(CREATION, ASSIGN_EXPERT,
                            ROUTING, CUSTOMER_NOTIFICATION);

// Ticket Routing has NO dependencies on any other component
ArchRule routingNoDeps = noClasses()
        .that().resideInAPackage(ROUTING)
        .should().dependOnClassesThat()
        .resideInAnyPackage(CREATION, COMPLETION, ASSIGN_EXPERT,
                            CUSTOMER_NOTIFICATION);

// Customer Notification has NO dependencies on any other component
ArchRule notificationNoDeps = noClasses()
        .that().resideInAPackage(CUSTOMER_NOTIFICATION)
        .should().dependOnClassesThat()
        .resideInAnyPackage(CREATION, COMPLETION,
                            ASSIGN_EXPERT, ROUTING);

completionNoDeps.check(importedClasses);
routingNoDeps.check(importedClasses);
notificationNoDeps.check(importedClasses);
When specifying component dependencies, watch out for generated code that specifies exclusive dependencies. For example, prompting an LLM to generate NetArchTest code in C# based on the assertion that the Ticket Creation component is only dependent on the Ticket Assignment component will sometimes yield the following code:

// ASSERT(Ticket Creation IS ONLY DEPENDENT ON Ticket Assignment)

var result = Types.InCurrentDomain()
   .That().ResideInNamespace(CreationNs)
    .Should().OnlyHaveDependenciesIn(       // ❶
        CreationNs,
        AssignExpertNs)
   .GetResult();

Assert.True(result.IsSuccessful, result.GetFailingTypes());
❶ Here is the problem; while this code correctly specifies the dependency that the Ticket Creation component can only instantiate classes within itself and the Ticket Assignment component, it also excludes all other library and framework classes (such as logging libraries, .Net framework utilities, and so on) from being instantiated. To prevent this, the architect could rewrite the assertion in the ADL to specify which components the Ticket Creation component cannot access:

// ASSERT(Ticket Creation has NO DEPENDENCY ON
//           Ticket Completion,
//           Ticket Routing,
//           Customer Notification)

var disallowed = new[] {
   CompletionNs,
   RoutingNs,
   CustomerNotificationNs
};

var result =Types.InCurrentDomain()
   .That().ResideInNamespace(CreationNs)
   .Should().NotHaveDependencyOnAny(disallowed)
   .GetResult();

Assert.True(result.IsSuccessful, result.GetFailingTypes());
Not all LLMs will generate code with this issue, but it's something to definitely watch out for.

Describing a domain in parts
Some of the issues with describing the entire Ticketing domain in a single ADL, as shown in Example 3-3, are readability and maintainability. Our example is a simple one, but for larger, more complex systems with big domains, such an ADL file can quickly become unmanageable and unreadable. Furthermore, any change to that ADL file would require an architect to regenerate the single class again, possibly applying tweaks to the generated source code.

An alternative, particularly for large domains, is to describe domains through separate ADL files. Doing so makes the ADL easier to understand and also helps facilitate maintainability, code regeneration, and direct source code modification. We recommend the following: * Describe the higher-level domain structure, including any components and subdomains, in one ADL file * Describe each subdomain in its own dedicated ADL file * Connect these files through the name of the ADL file or via custom tags.

For example, consider the Sysops Squad Ticketing domain. Rather than creating a single ADL file, as shown in Example 3-3, the architect could define the components and higher-level subdomains first, as shown in Example 3-4.

Example 3-4. ADL for describing the higher-level ticketing domain in Sysops Squad

FILENAME: ticketing_domain_primary.adl
DESCRIPTION Define Primary Ticketing Domain
CATEGORY Structural
DEFINE SYSTEM Sysops Squad AS sysops
  DEFINE DOMAIN Ticketing AS ticketing
    DEFINE COMPONENT Ticket Creation AS creation
    DEFINE COMPONENT Ticket Completion AS completion
    DEFINE SUBDOMAIN Assignment AS assignment
ASSERT(CLASSES are only CONTAINED within COMPONENTS and SUBDOMAINS)
Notice that in this ADL we did not describe the Assignment subdomain--only noted that it is part of the Ticketing domain. This ADL file makes it clear that the Ticketing domain consists of two components and one subdomain. From here, we can further define the subdomain details, as shown in Example 3-5:

Example 3-5. ADL for describing the assignment subdomain of the ticketing domain in Sysops Squad

FILENAME: ticketing_assignment_subdomain.adl
DESCRIPTION Define Assignment Subdomain of the Ticketing Domain
CATEGORY Structural
DEFINE SYSTEM Sysops Squad AS sysops
  DEFINE DOMAIN Ticketing AS ticketing
    DEFINE SUBDOMAIN Assignment AS assignment
       DEFINE COMPONENT Ticket Assignment AS assign_expert
       DEFINE COMPONENT Ticket Routing AS routing
       DEFINE COMPONENT Customer Notification AS customer_notification

ASSERT(CLASSES are only CONTAINED within COMPONENTS and SUBDOMAINS)
If the Assignment subdomain had further subdomains within it, this process would continue until only the components were left to define.

The trade-off here is that the Assignment subdomain ADL file must duplicate the definitions for the SYSTEM (Sysops Squad) and DOMAIN (Ticketing). This means that if the domain or system name changes, the architect will need to change it in multiple ADL files. We don't consider this a significant trade-off, since these artifacts don't change often. However, component dependencies that cross over into another subdomain can create more duplication.

In our logical architecture shown in [Link to Come], the Ticket Creation component has a dependency on the Ticket Assignment component within the Assignment subdomain. Here, the architect would need to define the Assignment subdomain and the Ticket Assignment component in the primary Ticketing domain ADL file if specifying the dependencies there, thus duplicating the definition of the subdomain and component within it. This modified ADL file is shown in Example 3-6:

Example 3-6. ADL for describing the ticketing domain in Sysops Squad with a Subdomain Dependency

FILENAME: ticketing_domain_primary.adl
DESCRIPTION Define Ticketing Domain
CATEGORY Structural
DEFINE SYSTEM Sysops Squad AS sysops
  DEFINE DOMAIN Ticketing AS ticketing
    DEFINE COMPONENT Ticket Creation AS creation
    DEFINE COMPONENT Ticket Completion AS completion
    DEFINE SUBDOMAIN Assignment AS assignment
      DEFINE COMPONENT Ticket Assignment AS assign_expert

ASSERT(CLASSES are only CONTAINED within COMPONENTS and SUBDOMAINS)

ASSERT(Ticket Creation IS DEPENDENT ON Ticket Assignment)
ASSERT(Ticket Completion has NO DEPENDENCY ON other components)
ASSERT(Ticket Assignment has NO DEPENDENCY ON Ticket Creation, Ticket Completion)
For smaller systems and domains, this duplication might lead an architect to describe the entire domain in a single ADL file, as shown in Example 3-3. However, for larger systems and more complex domains, this duplication is often worth the trade-off of having simpler, more narrowly defined ADL files, which makes it easier to maintain the corresponding generated source code.

Distributed architectures
For distributed architectures such as microservices, event-driven, and space-based architectures, the logical architecture and the physical architecture meet in the dependencies between components and the structural constraints regarding components and subdomains. When synchronous component dependencies deployed as separate deployed services are described in the logical architecture (such as that illustrated in Figure 2-2 in Chapter 2), this should not be indicated in the ADL assertions, because those assertions will enforce class file instantiation as opposed to service invocation. Rather, the assertions in the ADL provide dependency constraints between services. The ADL can then further define component dependencies within those services.

To illustrate this point, consider once again the logical architecture in Figure 2-2 in Chapter 2, which assumes a single deployment unit, from a governance standpoint. However, let's assume that this team uses a mono-repo and that its logical architecture is implemented through microservices. The Ticket Creation component, Ticket Completion component, and Assignment subdomain are deployed as separate services and generally defined through higher-level directory structures (which, in a monolithic system, would be the domains ). Therefore, the directory structure for the Ticketing domain for these three services might look like that shown in Figure 3-4.

This monolithic code structure matches the services defined for Sysops Squad Figure 3-4. This monolithic code structure matches the services defined for Sysops Squad

The Entry directory in each service directory is a component that contains the entrypoint class for the service: that is, the class containing the service's RESTful endpoints for the service and startup logic. The other directory structures under the service directory represent the components contained within the service. Notice that the service directories (those containing a service suffix) specify the deployment unit but still maintain the component-based structure described in Chapter 2.

The Assignment service, denoted by the ticket_assignment_service directory, contains five components. This is typical within a distributed architecture: each service, regardless of its size, still has an internal (and well-defined and well-structured) architecture and there is not always one-to-one correspondence between a service and a component. The Shared directory is a component containing code shared with other components within that service.

Architects could use ADL to define these services (and their corresponding components) just as they would in a monolithic system, except using the artifact SERVICE rather than DOMAIN. The ADL shown in Example 3-7, which corresponds to the prior directory structure, enforces isolation between services. This addresses some of the issues surrounding cross-service dependencies in mono-repo and repo-per-domain configurations, which we discuss in detail in Part II.

Example 3-7. ADL for describing the services within the Ticketing domain in Sysops Squad

DESCRIPTION Define Ticketing Domain and Services
CATEGORY Structural
DEFINE SYSTEM Sysops Squad AS sysops
  DEFINE DOMAIN Ticketing AS ticketing
    DEFINE SERVICE Ticket Creation Service as ticket_creation_service
      DEFINE COMPONENT Creation Entry AS entry
      DEFINE COMPONENT Creation AS creation
    DEFINE SERVICE Ticket Completion Service as ticket_creation_service
      DEFINE COMPONENT Completion Entry AS entry
      DEFINE COMPONENT Completion AS completion
    DEFINE SERVICE Ticket Assignment Service AS ticket_assignment_service
      DEFINE COMPONENT Assignment Entry AS entry
      DEFINE COMPONENT Ticket Assignment AS assign_expert
      DEFINE COMPONENT Ticket Routing AS routing
      DEFINE COMPONENT Customer Notification AS customer_notification
      DEFINE COMPONENT Assignment Shared AS shared

ASSERT(CLASSES are only CONTAINED within COMPONENTS and SUBDOMAINS)

ASSERT(Ticket Creation Service has NO DEPENDENCY ON other services)
ASSERT(Ticket Completion Service has NO DEPENDENCY ON other services)
ASSERT(Ticket Assignment Service has NO DEPENDENCY ON other services)
Prompting an LLM to generate source code in ArchUnit in Java for the ADL in Example 3-7 yields the following code for the dependencies between services:

// Ticket Creation Service should not depend on other services
ArchRule creationNoDepsOnOtherServices = noClasses()
     .that().resideInAPackage(CREATION_SERVICE)
     .should().dependOnClassesThat()
     .resideInAnyPackage(COMPLETION_SERVICE, ASSIGNMENT_SERVICE);

// Ticket Completion Service should not depend on other services
ArchRule completionNoDepsOnOtherServices = noClasses()
    .that().resideInAPackage(COMPLETION_SERVICE)
    .should().dependOnClassesThat()
    .resideInAnyPackage(CREATION_SERVICE, ASSIGNMENT_SERVICE);

// Ticket Assignment Service should not depend on other services
ArchRule assignmentNoDepsOnOtherServices = noClasses()
   .that().resideInAPackage(ASSIGNMENT_SERVICE)
   .should().dependOnClassesThat()
   .resideInAnyPackage(CREATION_SERVICE, COMPLETION_SERVICE);

creationNoDepsOnOtherServices.check(importedClasses);
completionNoDepsOnOtherServices.check(importedClasses);
assignmentNoDepsOnOtherServices.check(importedClasses);
For teams using a repo-per-service, the architect could still define the domain and corresponding services through ADL. However, they would not generate corresponding source code, since there is no single repository to enforce the rules. Instead, they would typically define a separate ADL file for each service that describes that service and its corresponding subdomains (if any). For example, to describe a dedicated repository containing the Ticket Creation service, an architect could write theADL shown in Example 3-8:

Example 3-8. ADL describing the Sysops Squad ticket-creation service for Sysops Squad using a repo-per-service

DESCRIPTION Define Ticket Creation Service
CATEGORY Structural
DEFINE SYSTEM Sysops Squad AS sysops
  DEFINE DOMAIN Ticketing AS ticketing
    DEFINE SERVICE Ticket Creation Service as ticket_creation_service
      DEFINE COMPONENT Creation Entry AS entry
      DEFINE COMPONENT Creation AS creation

ASSERT(CLASSES are only CONTAINED within COMPONENTS and SUBDOMAINS)
The line DEFINE DOMAIN Ticketing AS ticketing defines the domain to which this particular service belongs. We highly recommend this approach. Not only does it document the domain of the service, it creates consistent domain identification when using a repo-per-service configuration.

Reducing Friction
To align a system's implementation with its architecture, the structure of the source code--the directories, package names, and namespaces--must be aligned with the domains, subdomains, and components depicted in the system's logical or physical architecture. Predictably, this tends to create friction between architects and development teams.

Traditionally, software architecture and software development have been two separate activities. Software architects identify the system's critical architectural characteristics, create a logical architecture, make decisions regarding the most appropriate architecture style to use, and then identify the physical services for the system (assuming it's distributed). Development teams then take these artifacts and implement the architecture through class diagrams, UI design, and, of course, writing (or in some cases generating and editing) source code.

However, to govern the system's implementation and ensure it aligns with the architecture, the structure of the source code must match those of the logical and physical architecture, the ADL, and the generated executable source code used to detect gaps between the architecture and implementation (as shown in Figure 3-5). This pulls the software architect into the realm of implementation, which traditionally is the purview of developers. Many developers see this as intrusive, with architects "stealing" the art of programming away from them.

Artifact Alignment Figure 3-5. All four of these artifacts must align

One way to mitigate the eventual friction is for the architect to explain to the development team why this name alignment is necessary and provide concrete goals. Usually, development teams come up with the names of the directories or namespaces, providing they match the corresponding architecture. Then the architect can write the ADL to match those names.

We also recommend explaining to development teams that the ADL and the corresponding executable tests are not intended to control their actions, but merely be a placeholder for fast feedback when the implementation is misaligned with the designated architecture.

Iterating on the Architecture
A system's architecture always changes eventually, whether it's a greenfield (new) or brownfield (existing) system undergoing modifications. This is where Architecture as Code can help architects keep the architecture and implementation constantly aligned.

Refer back to the directory structure shown in Figure 3-3. Suppose the development team needs to add functionality to this system to modify or cancel an existing ticket. They create a new subdirectory under the ticket directory and name it modify_ticket. This new directory represents a new component in the architecture, so the implementation is now misaligned with the architecture.

However, because the architect created an ADL file (Example 3-3) and corresponding executable source code ([Link to Come], [Link to Come], and [Link to Come]), the test governing the Ticketing domain will fail, triggering a notification to the architect that the ADL, architecture diagram, and/or corresponding executable test are now out of sync with the implementation (or the other way around). They can now modify the architecture diagram to add the Ticket Modify component and update the ADL and test, so the architecture and implementation are constantly aligned.

A failed test derived from the ADL can also act as a placeholder when further conversation is needed. For example, if the development team creates a new component, the architect may consult with them to make sure that's the most appropriate action, as opposed to adding the functionality to an existing component. This helps foster a collaborative environment where the architect and development team are working together to iterate on the architecture.

Architects often delegate the work of modifying the architecture diagrams, ADL, and e tests to a lead developer. The architect has to trust the lead developer to make the right choices and to confer with the architect for advice and direction if they have a concern or are unsure about a particular modification. This can be a win-win approach; not only does it free the architect from continually modifying the architectural artifacts, more importantly, it gives the development team a better perspective on the underlying architecture of the system, so they can create a more effective software system.

Regenerate the source code or modify it?
When adding or removing components to or from an ADL file , the architect can regenerate the executable source code or directly modify the previously generated source code. Depending on the tool they use (ArchUnit, PyTestArch, NetArchTest, and so on), the generated source code will sometimes need modifications (believe it or not, generative AI is not perfect at generating source code). Therefore, unless the change is a massive architectural restructuring, we usually recommend directly modifying the existing executable source code when modifying the ADL file.

Aligning Existing Systems
For existing systems that become misaligned with the architecture (such as the one shown in Figure 2-7), architects can use the techniques described in this chapter to define an appropriate architecture, then start refactoring to properly align the system with the proposed architecture.

In Chapter 2 we introduced fitness-function-driven architecture (FFDA), where you start by defining the domains of the system, then further define the details of the most critical domains. This technique is similar to Test-Driven Development, where teams first write unit tests (which initially fail), then develop source code to make the tests pass.

By defining the ADL and corresponding executable source code for the proposed architecture, development teams can begin to refactor the structure of the source code to make those tests pass. In our book Software Architecture: The Hard Parts, we introduced several refactoring patterns to help architects and developers identify (and realize) a system's domains, subdomains, and components.

Specifically, we recommend using the Flattening pattern, described in Chapter 5 of that book, to refactor the system structure to match the ADL and tests. It describes pushing all source code down to the leaf nodes of the directory structure or namespace. Doing so creates definable components, with all source code contained within a component--since components are the building blocks of the architecture. From there, the architect can begin grouping components into subdomains, and then components and subdomains into domains.

Summary
The techniques described in this chapter are not meant to be treated like unit tests, where a failed test means something is wrong and needs to be addressed right away. Rather, they're meant to provide the fastest possible feedback loop to ensure that architecture and implementation are correctly aligned. Failed tests are a call to action to modify the ADL, architecture diagrams, and corresponding fitness functions.

It's possible to leverage the concept of architecture as code without using ADL as described in this chapter by writing the source code directly. However, using ADL as an interim artifact not only provides a platform-independent description of the architecture, it's much easier to read and provides a better understanding of the architecture.

This chapter has focused on the structural aspects of the intersection of architecture and implementation. The next chapter will continue by showing how to govern architectural constraints, another important aspect of aligning architecture with implementation.

Architectural Constraints
Architectural constraints provide development teams with guidelines as to what is and isn't allowed within the architecture. Constraints usually come from architectural decisions, such as "all communication between services will use request/reply messaging" or "the user interface cannot access the database directly." Constraints are typically documented through Architecture Decision Records (ADRs), which record the context, justifications, and consequences of a decision.1 While ADRs are a great way to document architecture decisions, they can't enforce them. Architectural decisions have always been a challenge for architects to enforce and govern--until now.

How Constraints Are Formed
Architects make architectural decisions based on specific business requirements, which form constraints in the architecture that generally support important architectural characteristics. If development teams don't implement those architectural constraints properly, the architecture might not (and usually doesn't) meet the needs and goals of the business. For this reason, architectural constraints must be governed in the same way as the structure of the system, as we discussed in the previous chapter. Fortunately, ADL can be used to describe architectural constraints, which architects can then use to generate (or write) executable source code for fitness functions to govern those constraints.

To illustrate how important constraints are to the success of an architecture, consider the following business need for a new system as specified by the business owner or sponsor:

We have a very tight budget and timeframe for this new product. In order to get this product out the door quickly, we're planning on using the relational database we have in place. However, we expect to experiment with other types of databases after the initial release. We need fast time to market when those database changes happen.

The architect, upon analyzing these very tight budget and timeframe requirements, realizes that cost and simplicity are critical for this new product. After performing a qualitative analysis (see Fundamentals of Software Architecture, 2nd edition), the architect sees that a monolithic architectural style is a likely candidate that would maximize these two architectural characteristics, which the more complex distributed architectures don't support well (see Figure 4-1).

Qualitative star ratings for cost and simplicity Figure 4-1. Qualitative star ratings for cost and simplicity

Two monolithic styles, the traditional n-tiered layered architecture and the modular monolith architecture, both fit the requirements well. The architect, unsure about which to choose, analyzes the last part of the requirement, which is about database changes. They notice that the layered architecture is technically partitioned: its primary overall structure is grouped by technical areas (such as the layers in a layered architecture). By contrast, the modular monolith style is domain- partitioned: it is primarily structured by domain area. For example, the modules in a modular monolith might contain domains like ordering, shipping, warehousing, and so on.

The architect realizes that using technical partitioning would isolate the database logic into a separate layer in the layered architecture, whereas domain partitioning would spread the database logic throughout each module in the entire architecture. Isolating the database logic into its own layer would allow the team to make quick database replacements without impacting other parts of the system, so the architect chooses the traditional n-tiered layered architecture as the most appropriate architecture for this new product.

The layered architecture would be a good choice for the business problem Figure 4-2. The layered architecture would be a good choice for the business problem

As Figure 4-2 shows, the layered architectural style supports a good separation of concerns and layers of isolation. This allows the database to be quickly and easily replaced with another database type, particularly since the objects representing the database entities wouldn't have to change--only the type of database used. Essentially, the Persistence layer would form an abstraction layer over the database, isolating the underlying database type (and its corresponding database structure) from the rest of the layers.

The concept of layers of isolation in layered architectures is an important one. Isolating layers within the architecture and creating well-defined contracts between those layers allows each layer to change independently from other layers. That reduces the system's overall brittleness and improves its agility (the ability to respond quickly to change). Layers of isolation are usually achieved through the concepts of open and closed layers. A closed layer in the architecture means that all requests from the layer above it must go through the layer below it, even if it's only a passthrough. An open layer means the layer above can either access the next layer in the architecture or step over it.

Figure 4-2 shows a good example of how to use open layers. To access the Persistence layer, the Business layer shouldn't have to go through the Services layer. Rather, it should be able to access the Persistence layer directly by bypassing the Services layer. To accomplish this, an architect would make the Services layer open so that it can be accessed or bypassed.

Figure 4-2 also shows a good example of closed layers. The Services layer contains all of the shared service functionality for the Business layer, but not_for the Presentation layer. Allowing the Presentation layer to access shared business services couples those layers together, meaning a change to a shared service for a business object would also require a change in objects in the Presentation layer. Therefore, to restrict the Presentation layer from accessing the shared Services layer, the architect would make the Business layer _closed. This means the Presentation layer must go through the Business layer when making a request, preventing access to the underlying Services layer.

Although the architect's choice of the layered architecture style is a good one, it will need certain constraints to achieve fast and easy database replacement and layers of isolation (thus making the architecture less brittle):

All database logic must be contained only within the Persistence layer.
No other layer besides the Persistence layer can access the database directly.
Open and closed layers must be respected, meaning the Presentation layer cannot talk to the Services layer or Persistence layer
No layer can be dependent on the one above it
These constraints, as Figure 4-3 shows, would allow the architecture to achieve the business goal of making it fast and easy to replace the database type, because the database logic is located in only one place in the architecture. Reducing coupling between the layers would also reduce brittleness,supporting the notion of isolation between all layers of the architecture.

Constraints within the layered architecture to support quick and easy database changes and layers of isolation Figure 4-3. Constraints within the layered architecture to support quick and easy database changes and layers of isolation

To document and communicate these architectural decisions, as mentioned, architects normally use ADRs. The following is an ADR for these decisions about database and database logic.

Example 4-1. ADR documenting and justifying the database-related decisions

ADR 22. All Database Logic Must Reside in the Persistence Layer

CONTEXT The business wants to experiment with different types of databases for the new product after its initial release and requires fast time to market when these database changes occur.

DECISION All database logic will reside only in the Persistence layer, meaning that no other layer can access the database directly or have any knowledge of the underlying database type and schema.

Isolating and containing the database logic solely to the Persistence layer ensures that replacing the database type will require changes to that layer only.

No other layers in the architecture will have any knowledge of the type of database used, nor of the underlying database structure.

Entity objects created by the Persistence layer and used by the other layers will not need to change, since such objects are different representations of the data than the database schema or structure.

The Persistence layer will act as the Database Abstraction layer, hiding the actual database implementation.

CONSEQUENCES All requests for data from the Presentation layer must go through all the layers in the architecture. This could introduce the Architecture Sinkhole antipattern, where each layer acts only as a passthrough for the requests without performing any additional processing.

Business logic will be separated from the corresponding database logic, making ongoing maintenance and testing more difficult.

Below is an ADR for the decision about open and closed layers and layer dependencies:

Example 4-2. ADR documenting and justifying layer dependencies and open/closed layers

ADR 23. Open And Closed Layers Must Be Respected

CONTEXT We want to isolate changes in the layers of the architecture to reduce brittleness and improve overall agility when adding or changing functionality.

DECISION Open and closed layers must be respected in the architecture, meaning the Presentation layer cannot access classes in the Services or Persistence layers. This means the Presentation layer will be closed so that all requests must flow through it first. The Business layer will be closed so that the Presentation layer cannot access the Services or Persistence layers. The Persistence layer will be closed, so that no layers under the Persistence layer (including the database) can be accessed by the Business or Services layers. Furthermore, no layer can be dependent on any layer above it.

Shared functionality in the Business layer will be contained within the Services layer and will be meant only for business objects contained within the Business layer. Restricting the Presentation layer from accessing the Services layer means that changes applied to the Services layer to accommodate shared business functionality will not impact the Presentation layer. Similarly, changes to the Persistence layer will only affect the Services and Business layers of the architecture, not the Presentation layer. This will reduce brittleness and improve overall agility in terms of maintenance and testing.

CONSEQUENCES All requests for data from the Presentation layer must go through all the layers in the architecture. This could introduce the Architecture Sinkhole antipattern, where each layer acts only as a passthrough for the requests without performing any additional processing. This could degrade responsiveness and increase the system's overall memory consumption.

Business logic would be separated from the corresponding database logic, making ongoing maintenance and testing more difficult.

The Presentation layer will have to create its own shared functionality, resulting in possible duplication of shared logic in the Presentation and Services layers.

With these architecture decisions documented and fully justified, the architect can now hand the architectural diagrams, ADRs, and business requirements over to the development team for implementation.

Misalignment
As we demonstrated in Chapter 3, it's easy for the implementation of an architecture to get misaligned with its structure. The same is true when it comes to architectural constraints. Let's continue our example from the previous section.

While implementing this new product, the UI team realizes that its responsiveness is too slow for simple queries. They also realize that it's taking way too long to coordinate with the backend business logic team to implement the Business layer and with the shared services team to implement the Services layer to create endpoints and contracts. The UI development team decides instead to call the database directly from the Presentation layer, reasoning that this will improve responsiveness and agility. (After all, the timeframe is very tight.) They also notice that the backend services team has already created a shared service in the Services layer to take care of formatting dates. The UI team also needs some of the date-formatting routines, so to speed up development, they decide to reuse this backend service, which communicates directly with those classes in the Services layer.

Along the same lines, the backend business logic and shared services teams are struggling because the business logic and database logic need to be developed together. Separating them is causing too much confusion and it's taking longer than necessary to implement the backend business functionality. The teams decide to combine most of the business logic and database logic together in the Business and Services layers, getting back on schedule to get the initial product out the door as quickly as possible.

The resulting architecture, as implemented by the development teams and released into production, is illustrated in Figure 4-4.

Resulting Layered Architecture Figure 4-4. The resulting architecture implemented by the development teams

Notice that every layer contains database logic, and each layer is accessing the database directly for some of its processing. Furthermore, the Presentation layer is bypassing the Business layer to access some of the shared services in the Services layer directly. The Presentation layer is also sometimes accessing the Persistence layer directly. While the development teams had good justifications for not following the architectural constraints (initial time to market pressures and responsiveness issues), this architecture won't meet the needs and goals of the business--and to see why, let's fast-forward a little.

Shortly after the initial implementation, the business decides that a key-value database would better suit the nature of the data for a certain domain area. However, because the development teams didn't follow the architectural constraints, the implementation doesn't align with the architecture, changing the database type will require changing all areas of the system, which the development teams deem too extensive and expensive to implement. Furthermore, the system is less reliable, because changes to the Services and Persistence layers keep breaking the Presentation layer. Fixing this requires additional time and more coordination between teams.

Like the structural aspects of architecture, architectural constraints must be governed to ensure that the implementation is aligned with the architecture. This is where architecture as code and ADL can help.

Aligning Constraints on Layer Communication
One of the architectural constraints in this example is that the open and closed layers of the architecture have to be respected. This means that, when implementing the architecture, development teams can't access certain layers from other layers. The following rules describing the open and closed layers (as shown in Figure 4-3) are part of that constraint:

The Presentation layer can only access the Business layer
The Business layer can only access the Services layer or the Persistence layer
The Services layer can only access the Persistence layer
The Persistence layer cannot access any other layer
These rules ensure that dependencies always flow down the layers and never up. They govern communication between the layers of the architecture. This constraint can also be described using the ADL shown in Example 4-3:

Example 4-3. ADL for describing open and closed layers in a layered architecture

DESCRIPTION Define Open/Closed Layers
CATEGORY Structural
DEFINE SYSTEM New Product AS new_product
  DEFINE LAYER Presentation Layer AS presentation
  DEFINE LAYER Business Layer AS business
  DEFINE LAYER Services Layer AS services
  DEFINE LAYER Persistence Layer AS persistence
ASSERT(Presentation Layer CAN ONLY ACCESS Business Layer)
ASSERT(Business Layer CAN ONLY ACCESS Services Layer, Persistence Layer)
ASSERT(Services Layer CAN ONLY ACCESS Persistence Layer)
ASSERT(Persistence Layer CANNOT ACCESS any other layer)
This ADL describes the architecture's overall structure with respect to open and closed layers. It can be used in conjunction with an LLM to generate an executable test using any of a variety of languages and architecture testing tools (as listed in [Link to Come]). For example, prompting an LLM to generate corresponding ArchUnit code in Java using the ADL in Example 4-3 yields the following generated code (abbreviated for brevity):

ArchRule layered_architecture_rules = layeredArchitecture()
  .layer("Presentation").definedBy("com.newproduct.presentation..")
  .layer("Business").definedBy("com.newproduct.business..")
  .layer("Services").definedBy("com.newproduct.services..")
  .layer("Persistence").definedBy("com.newproduct.persistence..")

  .whereLayer("Presentation").mayOnlyAccessLayers("Business")
  .whereLayer("Business").mayOnlyAccessLayers("Services", "Persistence")
  .whereLayer("Services").mayOnlyAccessLayers("Persistence")
  .whereLayer("Persistence").mayNotAccessAnyLayer();
In the above code snippet, the LLM chose to use the ArchUnit layered architecture API when generating the source code corresponding to the ADL. The LAYER definitions in the ADL triggered this choice.

Executing the same ADL and LLM prompt to generate a test in NetArchTest in C# yields the following generated code. NetArchTest doesn't have the same layered architecture API as ArchUnit does, so the LLM chose the core NetArchTest API instead (abbreviated for brevity):

private const string Presentation = "NewProduct.Presentation";
private const string Business = "NewProduct.Business";
private const string Services = "NewProduct.Services";
private const string Persistence = "NewProduct.Persistence";

/// Presentation Layer dependencies
var result = Types.InCurrentDomain()
  .That().ResideInNamespace(Presentation, true)
  .ShouldNot().HaveDependencyOnAny(Services,Persistence)
  .GetResult();
Assert.True(result.IsSuccessful, result.GetFailingTypes());

/// Business Layer dependencies
var result = Types.InCurrentDomain()
  .That().ResideInNamespace(Business, true)
  .ShouldNot().HaveDependencyOnAny(Presentation)
  .GetResult();
Assert.True(result.IsSuccessful, result.GetFailingTypes());
...
The Business layer dependency in the above code avoids dependencies in any higher layers (like the Presentation layer), meaning while the Presentation layer can access classes in the Business layer, the reverse is not allowed. This ensures full isolation of each layer, reduces coupling and dependencies, and guarantees that all dependencies flow only downward, never upward. By generating code using the ADL, the architect can not only describe the architectural constraints but enforce and govern them using executable source code.

Aligning Database Constraints
Restricting all database logic to the Persistence layer and making sure that no other layer has any database-related logic may sound tricky to describe in ADL, but it's surprisingly easy. Since connecting to a database is considered database logic, we can combine these two constraints into a single rule:

The Persistence layer is the only layer that contains database logic.

In creating the ADL for this architectural constraint, the architect can define each layer, then assert that only the Persistence layer contains database logic. It might be tempting to list all of the classes for the architecture's particular programming language in the assertions, such as in the following ADL describing a Java implementation:

Example 4-4. ADL for restricting specific database-related classes to the Persistence layer only

DESCRIPTION Restrict database access and database logic
CATEGORY Structural
DEFINE SYSTEM New Product AS new_product
  DEFINE LAYER Presentation Layer AS presentation
  DEFINE LAYER Business Layer AS business
  DEFINE LAYER Services Layer AS services
  DEFINE LAYER Persistence Layer AS persistence
  DEFINE DBCLASS Connection as java.sql.Connection.class
  DEFINE DBCLASS Statement as java.sql.Statement.class
  DEFINE DBCLASS ResultSet as java.sql.Resultset.class
ASSERT(only CLASSES in the Persistence Layer SHOULD CONTAIN DBCLASSES)
While an LLM would correctly generate the Java code from this ADL to make sure no other layer contains these classes (and hence database logic), two problems present themselves. First, the architect might not list every class that could indicate database logic. For example, in Example 4-4, the architect has failed to include the java.sql.PreparedStatement class as part of the DBCLASS definitions. Second, the ADL would then be language-specific, and as such, prone to API-class changes in that programming language.

A much better approach is to simply specify the basic constraint in the ASSERT verb in the ADL, as shown in Example 4-5, and let an LLM determine what classes constitute database logic for that particular programming language.

Example 4-5. ADL for restricting database logic to the Persistence layer only

DESCRIPTION Restrict database access and database logic
CATEGORY Structural
DEFINE SYSTEM New Product AS new_product
  DEFINE LAYER Presentation Layer AS presentation
  DEFINE LAYER Business Layer AS business
  DEFINE LAYER Services Layer AS services
  DEFINE LAYER Persistence Layer AS persistence
ASSERT(Persistence Layer is the only layer that contains database logic)
This ADL shows the power of using pseudocode and plain English to describe conditions and constraints. For example, prompting an LLM to generate ArchUnit code in Java produces the following results:

ArchRule rule = ArchRuleDefinition.noClasses()
  .that().resideOutsideOfPackage(BASE_PACKAGE + ".persistence..")
  .should().dependOnClassesThat().areAssignableTo(java.sql.Connection.class)
  .orShould().dependOnClassesThat().areAssignableTo(java.sql.Statement.class)
  .orShould().dependOnClassesThat().areAssignableTo(java.sql.PreparedStatement.class)
  .orShould().dependOnClassesThat().areAssignableTo(java.sql.ResultSet.class)
  .because("Only the Persistence layer should use JDBC/database types.");

rule.check(importedClasses);
Based on the assertion that the Persistence layer is the only layer that can contain database logic, the LLM has determined the correct Java classes for that specific programming language and automatically added them to the generated code. The same would be true if the LLM were prompted to generate ArchUnitNET code in C#:

string[] dbNamespaces = {
  "System.Data",
  "System.Data.SqlClient",
  "Microsoft.Data.SqlClient",
  "System.Data.Common"
};

var result = Types.InCurrentDomain().That()
   .DoNotResideInNamespace(Persistence, true)
   .ShouldNot().HaveDependencyOnAny(dbNamespaces)
   .GetResult();

Assert.True(result.IsSuccessful, result.GetFailingTypes());
These examples illustrate the power of ADL and architecture as code. The ADL provides clear, easily understood documentation of the constraint and can be used to generate executable source code to govern that constraint, without the architect having to supply the LLM with a lot of details.

Summary
The governance techniques described in this and the preceding chapter are meant to provide architects with the fastest possible feedback loop to ensure that architecture and implementation are correctly aligned. Architects should use failed tests as calls to action to modify the ADL, architecture diagrams, and corresponding executable fitness functions so that they correctly reflect the current state of the architecture. However, these techniques don't address the root cause of misalignment, which is usually a lack of communication and collaboration between software architects and development teams.

Governing architectural and structural constraints is a critical part of making architecture work and aligning implementation with architecture. Organizations can leverage ADL and the architecture-as-code concept to help foster stronger communication and collaboration between architects and development teams. Utilizing the fast feedback loop provided by executable fitness functions, they can work together almost in real time to align the implementation with the architecture and facilitate any possible changes to the architecture that arise as a result of learning more about the problem during implementation. To quote Fundamentals of Software Architecture: "Developers should never take components designed by architects as the last word. Rather, they should view the initial design as a first draft, where implementation will reveal more details and refinements."

As developers add or remove components, subdomains, services, and so on, they are effectively changing the architecture of the system. That's why they should collaborate with architects to make sure they're adding or removing the right architectural artifacts--and that those changes aren't degrading any of the system's architectural characteristics. When teams use failed tests as prompts for further discussion, using architecture as code, ADL, and executable fitness functions helps bridge this collaboration gap.

Remember our example of the UI team's decision to access the database directly? If that team was using architecture as code, ADL, and fitness functions, this action would fail a test, which would trigger a discussion between the architect and the UI team about the problem. This gives them all a chance to collaborate on other ways to improve responsiveness, such as leveraging caching. Without architecture as code and governance tests in place, that conversation might never happen.

Implementation is not the only intersection architects need to be concerned about. Each part of this book deals with an architectural intersection. Some may not seem as obvious as this one, but they are just as critical.